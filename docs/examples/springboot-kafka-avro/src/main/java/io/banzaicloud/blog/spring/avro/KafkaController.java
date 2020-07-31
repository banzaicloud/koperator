package io.banzaicloud.blog.spring.avro;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import io.banzaicloud.blog.User;

@RestController
@RequestMapping(value = "/user")
public class KafkaController {

  private final Producer producer;

  @Autowired
  KafkaController(Producer producer) {
    this.producer = producer;
  }

  @PostMapping(value = "/publish")
  public void sendMessageToKafkaTopic(@RequestParam("name") String name, @RequestParam("age") Integer age) {
    this.producer.sendMessage(new User(name, age));
  }
}
