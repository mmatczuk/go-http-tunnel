BENCHMARK_HOST     ?= localhost
BENCHMARK_PORT     ?= 8443
BENCHMARK_ADDR     ?= localhost:3131
BENCHMARK_DURATION ?= 2m
BENCHMARK_RATES    ?= 300 400 500

BENCHMARK_WARM_UP_DURATION ?= 15s
BENCHMARK_WARM_UP_RATE     ?= 100

BENCHMARK_TLS_CERT_FILE ?= ../test-fixtures/selfsigned.crt
BENCHMARK_TLS_KEY_FILE  ?= ../test-fixtures/selfsigned.key
BENCHMARK_TLS_CERT_ID   ?= YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4

############
# Building #
############

all: clean build

build: tunnelserver tunnelclient kodingtunnelserver kodingtunnelclient hdr

tunnelserver:
	go build ./cmd/tunnelserver

tunnelclient:
	go build ./cmd/tunnelclient

kodingtunnelserver:
	go build ./cmd/kodingtunnelserver

kodingtunnelclient:
	go build ./cmd/kodingtunnelclient

hdr:
	go build ./cmd/hdr

clean:
	@rm -f tunnelserver
	@rm -f tunnelclient
	@rm -f kodingtunnelserver
	@rm -f kodingtunnelclient
	@rm -f hdr

###########
# Running #
###########

run-tunnelserver: tunnelserver
	@./tunnelserver \
	-https :${BENCHMARK_PORT} \
	-addr ${BENCHMARK_ADDR} \
	-host ${BENCHMARK_HOST} \
	-clientid ${BENCHMARK_TLS_CERT_ID} \
	-tlscertfile ${BENCHMARK_TLS_CERT_FILE} \
	-tlskeyfile ${BENCHMARK_TLS_KEY_FILE}

run-tunnelclient: tunnelclient data
	@./tunnelclient \
	-serveraddr ${BENCHMARK_ADDR} \
	-datadir data \
	-tlscertfile ${BENCHMARK_TLS_CERT_FILE} \
	-tlskeyfile ${BENCHMARK_TLS_KEY_FILE}

run-kodingtunnelserver: kodingtunnelserver
	@./kodingtunnelserver \
	-https :${BENCHMARK_PORT} \
	-addr ${BENCHMARK_ADDR} \
	-host ${BENCHMARK_HOST} \
	-tlscertfile ${BENCHMARK_TLS_CERT_FILE} \
	-tlskeyfile ${BENCHMARK_TLS_KEY_FILE

run-kodingtunnelclient: kodingtunnelclient data
	@./kodingtunnelclient \
	-identifier ${BENCHMARK_HOST} \
	-serveraddr ${BENCHMARK_ADDR} \
	-datadir data

################
# Benchmarking #
################

attack: $(BENCHMARK_RATES:%=attack-%)
attack-%: targets
	@[ -n "${TAG}" ] || (echo "Please set a TAG, ie. make attack TAG=foo"; false)

	@echo "$(shell date +%R) > Warming up for ${BENCHMARK_WARM_UP_DURATION} at ${BENCHMARK_WARM_UP_RATE} req/s"
	@< targets vegeta attack -duration=${BENCHMARK_WARM_UP_DURATION} -rate=${BENCHMARK_WARM_UP_RATE} -insecure > /dev/null

	@echo "$(shell date +%R) > Running benchmark for ${BENCHMARK_DURATION} at $* req/s"
	@[ -d results ] || mkdir results
	@< targets vegeta attack -duration=${BENCHMARK_DURATION} -rate=$* -insecure > results/${TAG}-$*.bin
	@< results/${TAG}-$*.bin vegeta report
	@< results/${TAG}-$*.bin vegeta dump | ./hdr -rate=$* > results/${TAG}-$*.hdrc

targets: data
	@for i in data/*; do echo GET https://${BENCHMARK_HOST}:${BENCHMARK_PORT}/$$i; done | shuf > targets

data:
	tar -xjf data.tar.bz2

###########
# Cleanup #
###########

mrproper: clean
	@rm -f targets
	@rm -rf data
	@rm -rf results
