**DOCKER SCION**

The Makefile defines all the make commands used to build, run the images/containers. Docker-compose defines all the services and networks required. 

**base-isd**

Contains base Dockerfile and configuration files (br.toml, cs.toml) which are used as the basis for all other Docker images (e.g. scion01, scion02, ...).

Systemd folder contains the scion services (see https://systemd.io/). 

The pki-generation-isd0%i.bash files runs the certificate generation and signing ceremony for each respective isd. 

**scion0%i**

Contains topology files and an additional Dockerfile. 

**Monitor**

Currently not used. Ignore.



**SETUP**

I used WSL 2 Ubuntu with Docker Engine installed on Windows 11.
Unix should work too I think, just try. To start, simply run

```bash
make up
```

in the terminal and to stop
```bash
make down
```

**TODO**:
- Get HTTP API to work for tcpdump logging and file sending (see censorshipControl/node-capture.go and censorshipControl/node-capture-server.go)
- Extend to 4 ISD setup (new pki-generation file, new services/Dockerfile/Topology files, etc.)
- Maybe clean up and reformat, new folder structure etc.
- ?

To build the gofiles for the docker containers use:

```bash
env GOOS=linux GOARCH=amd64 go build -o <outputFileName> <goFile>
```

