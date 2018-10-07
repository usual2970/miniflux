module miniflux.app

// +heroku goVersion go1.11

require (
	github.com/PuerkitoBio/goquery v1.4.1
	github.com/andybalholm/cascadia v1.0.0 // indirect
	github.com/golang/protobuf v1.1.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/lib/pq v1.0.0
	golang.org/x/crypto v0.0.0-20180820150726-614d502a4dac
	golang.org/x/net v0.0.0-20180826012351-8a410e7b638d
	golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
	golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f // indirect
	golang.org/x/sys v0.0.0-20180824143301-4910a1d54f87 // indirect
	golang.org/x/text v0.3.0 // indirect
	google.golang.org/appengine v1.1.0 // indirect
)

replace (
	golang.org/x/crypto => github.com/golang/crypto v0.0.0-20180820150726-614d502a4dac
	golang.org/x/net => github.com/golang/net v0.0.0-20180826012351-8a410e7b638d
	golang.org/x/oauth2 => github.com/golang/oauth2 v0.0.0-20180821212333-d2e6202438be
	golang.org/x/sync => github.com/golang/sync v0.0.0-20180314180146-1d60e4601c6f // indirect
	golang.org/x/sys => github.com/golang/sys v0.0.0-20180824143301-4910a1d54f87 // indirect
	golang.org/x/text => github.com/golang/text v0.3.0 // indirect
	google.golang.org/appengine => github.com/golang/appengine v1.1.0 // indirect

)
