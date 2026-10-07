all: geoclient geography

debug: geoclient_debug geography_debug


geography:
	go build -o geography  ./gserver/

geoclient:
	go build -o  geoclient ./client/

geography_debug:
		go build -gcflags="all=-N -l" -o geography ./gserver/
geoclient_debug:
	go build -gcflags="all=-N -l" -o  geoclient ./client/
clean:
	rm geoclient
	rm geography
