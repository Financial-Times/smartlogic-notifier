module github.com/Financial-Times/smartlogic-notifier

go 1.13

require (
	github.com/Financial-Times/go-fthealth v0.0.0-20200609161010-4c53fbef65fa
	github.com/Financial-Times/go-logger/v2 v2.0.1
	github.com/Financial-Times/http-handlers-go v0.0.0-20170809121007-229ac16f1d9e
	github.com/Financial-Times/kafka-client-go v0.0.0-20181214120216-c3a1941e42a4
	github.com/Financial-Times/service-status-go v0.0.0-20200609183459-3c8b4c6d72a5
	github.com/Financial-Times/transactionid-utils-go v0.2.0
	github.com/gorilla/handlers v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/ivan-p-nikolov/jeager-service-example v0.0.0-20210817100032-de9010c73cdc
	github.com/jawher/mow.cli v1.1.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/sethgrid/pester v0.0.0-20170408212409-4f4c0a67b649
	github.com/sirupsen/logrus v1.0.5
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/otel v1.0.0-RC2
	golang.org/x/time v0.0.0-20190921001708-c4c64cad1fd0 // indirect
)

replace github.com/Financial-Times/kafka-client-go v0.0.0-20181214120216-c3a1941e42a4 => github.com/ivan-p-nikolov/kafka-client-go v1.1.0
