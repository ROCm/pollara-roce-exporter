# Pollara exporter

## About 
This is a Pollara 400 series NIC (AINIC) exporter that reads all of the counters and gauges from
`/sys/class/infiniband/ionic_*/ports/1/hw_counters`. 

* Counters are values that increase over time, starting at 0 and incrementing, such as `tx_bytes`. 
* Gauges are values that can increase or decrease over time, reflecting a value at specific point in time.
* The exporter is served out of port 9102. If you'd like a different port, you can update the last two lines at the
bottom of ```main.go``` with a new value, otherwise you can select your port if you are using Docker. 

## Install Go
Verify that you you have Go 1.20+ on your system with ```go version```. 

* If you need to install Go, run these commands:

    ```
    wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    go version
    ```
   
* If you have an older version of Go and need to upgrade:
    
    ```
    apt remove golang-go -y
    rm -rf /usr/local/go
    wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    go version
    ```

## Create exporter binary
Either `git clone` this repo or create a directory such as `pollara_exporter`.

    mkdir pollara_exporter
    cd pollara_exporter
    go mod init pollara_exporter

Then run `vi main.go` and copy main.go from this repo into your directory. 

## Build
    go build -o pollara_exporter
    
If you get a `no required module provides package github.com/prometheus/client_golang/prometheus` error, then run the
following commands to install them: 

    go get github.com/prometheus/client_golang/prometheus 
    go get github.com/prometheus/client_golang/prometheus/promhttp

Then re-run the ```go build -o pollara_exporter``` command. 

    ./pollara_exporter

The exporter is served out of port 9102. 

## Run in a Docker container 
If you are building this for CentOS, use the Dockerfile called ```Dockerfile_centos```, but remove `_centos` from the
filename before running so it just says `Dockerfile`. 

If you did a `git clone` to copy the directory structure, you'll already have `Dockerfile` present. If you copied
`main.go` from this repo, then copy `Dockerfile` into same directory as `main.go`. 

    docker build -t pollara-exporter .
    docker run -d --name pollara-exporter -p 9102:9102 --restart always --privileged -v /sys:/sys:ro pollara-exporter 
    docker ps
    curl localhost:9102/metrics

