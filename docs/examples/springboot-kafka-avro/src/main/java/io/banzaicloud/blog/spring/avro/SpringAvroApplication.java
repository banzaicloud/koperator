package io.banzaicloud.blog.spring.avro;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;

@SpringBootApplication
public class SpringAvroApplication {

  @Value("${topic.name}")
  private String topicName;

  @Value("${topic.partitions-num}")
  private Integer partitions;

  @Value("${topic.replication-factor}")
  private short replicationFactor;

  public static void main(String[] args) {
    SpringApplication.run(SpringAvroApplication.class, args);
  }

  @Bean
  NewTopic moviesTopic() {
    return new NewTopic(topicName, partitions, replicationFactor);
  }
}


