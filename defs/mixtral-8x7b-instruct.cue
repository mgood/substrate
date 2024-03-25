package defs

import (
  "github.com/ajbouh/substrate/defs/chat:chat_completion"
)

enable: "mixtral-8x7b-instruct": true

tests: "mixtral-8x7b-instruct": assister: {
  test_templates["assister"]

  environment: URL: "http://substrate:8080/mixtral-8x7b-instruct/v1"
  depends_on: "substrate": true
}

services: "mixtral-8x7b-instruct": {
  spawn: {
    image: images["vllm"]
    environment: {
      CUDA_DEVICE_ORDER: "PCI_BUS_ID"
    }

    resourcedirs: {
      model: {
        id: "huggingface:model:casperhansen/mixtral-instruct-awq:0a898130957afe22021bbaf807f50f6bbce88201"
      }
    }

    command: [
      "--host", "0.0.0.0",
      "--port", "8080",
      "--model=/res/model/huggingface/local",
      "--enforce-eager",
    ]
  }
}
