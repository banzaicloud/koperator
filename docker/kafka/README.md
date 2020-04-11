# Kafka docker image

`adobe/kafka` docker image build configuration.

A new `kafka-*` tag created in this repo triggers the image build and push to [adobe/kafka](https://hub.docker.com/r/adobe/kafka/tags?page=1&ordering=last_updated) docker hub repo.

Tags should be `kafka-<scala_version>-<kafka_version>` e.g `kafak-2.13-2.6.2`

# Upstream base

This is based on [wurstmeister/kafka-docker](https://github.com/wurstmeister/kafka-docker) with the following additions:
1. Use `openjdk 16` in order to support container based resource monitoring
2. Use custom `log4j.properties` to get all kafka logs to stdout only
