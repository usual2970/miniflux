// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package storage // import "miniflux.app/storage"

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq/hstore"
	"regexp"
	"time"

	"miniflux.app/logger"
	"miniflux.app/model"
	"miniflux.app/timer"

	"github.com/lib/pq"
)

// CountUnreadEntries returns the number of unread entries.
func (s *Storage) CountUnreadEntries(userID int64) int {
	builder := s.NewEntryQueryBuilder(userID)
	builder.WithStatus(model.EntryStatusUnread)

	n, err := builder.CountEntries()
	if err != nil {
		logger.Error("unable to count unread entries: %v", err)
		return 0
	}

	return n
}

// NewEntryQueryBuilder returns a new EntryQueryBuilder
func (s *Storage) NewEntryQueryBuilder(userID int64) *EntryQueryBuilder {
	return NewEntryQueryBuilder(s, userID)
}

// UpdateEntryContent updates entry content.
func (s *Storage) UpdateEntryContent(entry *model.Entry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE entries SET content=$1 WHERE id=$2 AND user_id=$3`, entry.Content, entry.ID, entry.UserID)
	if err != nil {
		tx.Rollback()
		return err
	}

	query := `
		UPDATE entries
		SET document_vectors = to_tsvector(title || ' ' || coalesce(content, ''))
		WHERE id=$1 AND user_id=$2
	`
	_, err = tx.Exec(query, entry.ID, entry.UserID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// createEntry add a new entry.
func (s *Storage) createEntry(entry *model.Entry) error {
	query := `
		INSERT INTO entries
		(title, hash, url, comments_url, published_at, content, author, user_id, feed_id, document_vectors, extra)
		VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, to_tsvector($1 || ' ' || coalesce($6, '')), $10)
		RETURNING id, status
	`

	extra := hstore.Hstore{Map: make(map[string]sql.NullString)}
	extra.Map["img"] = sql.NullString{String: getImg(entry.Content), Valid: true}
	
	err := s.db.QueryRow(
		query,
		entry.Title,
		entry.Hash,
		entry.URL,
		entry.CommentsURL,
		entry.Date,
		entry.Content,
		entry.Author,
		entry.UserID,
		entry.FeedID,
		extra,
	).Scan(&entry.ID, &entry.Status)

	if err != nil {
		return fmt.Errorf("unable to create entry: %v", err)
	}

	for i := 0; i < len(entry.Enclosures); i++ {
		entry.Enclosures[i].EntryID = entry.ID
		entry.Enclosures[i].UserID = entry.UserID
		err := s.CreateEnclosure(entry.Enclosures[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// updateEntry updates an entry when a feed is refreshed.
// Note: we do not update the published date because some feeds do not contains any date,
// it default to time.Now() which could change the order of items on the history page.
func (s *Storage) updateEntry(entry *model.Entry) error {
	query := `
		UPDATE entries SET
		title=$1, url=$2, comments_url=$3, content=$4, author=$5,
		document_vectors=to_tsvector($1 || ' ' || coalesce($4, ''))
		WHERE user_id=$6 AND feed_id=$7 AND hash=$8
		RETURNING id
	`
	err := s.db.QueryRow(
		query,
		entry.Title,
		entry.URL,
		entry.CommentsURL,
		entry.Content,
		entry.Author,
		entry.UserID,
		entry.FeedID,
		entry.Hash,
	).Scan(&entry.ID)

	if err != nil {
		return err
	}

	for _, enclosure := range entry.Enclosures {
		enclosure.UserID = entry.UserID
		enclosure.EntryID = entry.ID
	}

	return s.UpdateEnclosures(entry.Enclosures)
}

// entryExists checks if an entry already exists based on its hash when refreshing a feed.
func (s *Storage) entryExists(entry *model.Entry) bool {
	var result int
	query := `SELECT count(*) as c FROM entries WHERE user_id=$1 AND feed_id=$2 AND hash=$3`
	s.db.QueryRow(query, entry.UserID, entry.FeedID, entry.Hash).Scan(&result)
	return result >= 1
}

// cleanupEntries deletes from the database entries marked as "removed" and not visible anymore in the feed.
func (s *Storage) cleanupEntries(feedID int64, entryHashes []string) error {
	query := `
		DELETE FROM entries
		WHERE feed_id=$1 AND
		id IN (SELECT id FROM entries WHERE feed_id=$2 AND status=$3 AND NOT (hash=ANY($4)))
	`
	if _, err := s.db.Exec(query, feedID, feedID, model.EntryStatusRemoved, pq.Array(entryHashes)); err != nil {
		return fmt.Errorf("unable to cleanup entries: %v", err)
	}

	return nil
}

// UpdateEntries updates a list of entries while refreshing a feed.
func (s *Storage) UpdateEntries(userID, feedID int64, entries model.Entries, updateExistingEntries bool) (err error) {
	var entryHashes []string
	for _, entry := range entries {
		entry.UserID = userID
		entry.FeedID = feedID

		if s.entryExists(entry) {
			if updateExistingEntries {
				err = s.updateEntry(entry)
			}
		} else {
			err = s.createEntry(entry)
		}

		if err != nil {
			return err
		}

		entryHashes = append(entryHashes, entry.Hash)
	}

	if err := s.cleanupEntries(feedID, entryHashes); err != nil {
		logger.Error("[Storage:CleanupEntries] %v", err)
	}

	return nil
}

// ArchiveEntries changes the status of read items to "removed" after 60 days.
func (s *Storage) ArchiveEntries() error {
	query := `
		UPDATE entries SET status='removed'
		WHERE id=ANY(SELECT id FROM entries WHERE status='read' AND starred is false AND published_at < now () - '60 days'::interval LIMIT 500)
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("unable to archive read entries: %v", err)
	}

	return nil
}

// SetEntriesStatus update the status of the given list of entries.
func (s *Storage) SetEntriesStatus(userID int64, entryIDs []int64, status string) error {
	defer timer.ExecutionTime(time.Now(), fmt.Sprintf("[Storage:SetEntriesStatus] userID=%d, entryIDs=%v, status=%s", userID, entryIDs, status))

	query := `UPDATE entries SET status=$1 WHERE user_id=$2 AND id=ANY($3)`
	result, err := s.db.Exec(query, status, userID, pq.Array(entryIDs))
	if err != nil {
		return fmt.Errorf("unable to update entries status: %v", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to update these entries: %v", err)
	}

	if count == 0 {
		return errors.New("nothing has been updated")
	}

	return nil
}

// ToggleBookmark toggles entry bookmark value.
func (s *Storage) ToggleBookmark(userID int64, entryID int64) error {
	defer timer.ExecutionTime(time.Now(), fmt.Sprintf("[Storage:ToggleBookmark] userID=%d, entryID=%d", userID, entryID))

	query := `UPDATE entries SET starred = NOT starred WHERE user_id=$1 AND id=$2`
	result, err := s.db.Exec(query, userID, entryID)
	if err != nil {
		return fmt.Errorf("unable to toggle bookmark flag: %v", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to toogle bookmark flag: %v", err)
	}

	if count == 0 {
		return errors.New("nothing has been updated")
	}

	return nil
}

// FlushHistory set all entries with the status "read" to "removed".
func (s *Storage) FlushHistory(userID int64) error {
	defer timer.ExecutionTime(time.Now(), fmt.Sprintf("[Storage:FlushHistory] userID=%d", userID))

	query := `UPDATE entries SET status=$1 WHERE user_id=$2 AND status=$3 AND starred='f'`
	_, err := s.db.Exec(query, model.EntryStatusRemoved, userID, model.EntryStatusRead)
	if err != nil {
		return fmt.Errorf("unable to flush history: %v", err)
	}

	return nil
}

// MarkAllAsRead set all entries with the status "unread" to "read".
func (s *Storage) MarkAllAsRead(userID int64) error {
	defer timer.ExecutionTime(time.Now(), fmt.Sprintf("[Storage:MarkAllAsRead] userID=%d", userID))

	query := `UPDATE entries SET status=$1 WHERE user_id=$2 AND status=$3`
	_, err := s.db.Exec(query, model.EntryStatusRead, userID, model.EntryStatusUnread)
	if err != nil {
		return fmt.Errorf("unable to mark all entries as read: %v", err)
	}

	return nil
}

// EntryURLExists returns true if an entry with this URL already exists.
func (s *Storage) EntryURLExists(userID int64, entryURL string) bool {
	var result int
	query := `SELECT count(*) as c FROM entries WHERE user_id=$1 AND url=$2`
	s.db.QueryRow(query, userID, entryURL).Scan(&result)
	return result >= 1
}

func getImg(html string) string {
	reg := regexp.MustCompile(`img\\s+src="(.*?)"`)
	rs := reg.FindStringSubmatch(html)
	if len(rs) == 0 {
		return ""
	}
	return rs[1]
}
