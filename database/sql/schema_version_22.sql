alter table entries add column extra hstore;
create index entries_extra_idx on entries using gin(extra);