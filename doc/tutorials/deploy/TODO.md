**TODO**:
- [x] (incomplete) Get HTTP API to work for tcpdump logging and file sending (see censorshipControl/node-capture.go and censorshipControl/node-capture-server.go)
- [ ] Basic HTTP API to control scion services (e.g. systemctl)
- [x] Extend to 4 ISD setup (new pki-generation file, new services/Dockerfile/Topology files, etc.)
- [ ] Maybe clean up and reformat, new folder structure etc.
- [ ] Check if we can adapt scion webapp to run on our local network
- [x] Run a small webserver (see [here](https://github.com/netsec-ethz/scion-apps/tree/55667b489898af09ae9d8290410da0be176549f9/_examples/shttp/server)) and install scion-apps with scion-bat
- [ ] Draw diagram of full topology 
- [ ] Adapt Makefile to run without building anew everytime
- [ ] Fix: When running make up after NOT running make purge, there is an error with the pki-generation scripts