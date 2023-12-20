package dev

import (
  asr "github.com/ajbouh/substrate/pkg/asr"
)

enable: "asr-faster-whisper": true

imagespecs: "asr-faster-whisper": {}

// default model choices are
// "Systran/faster-whisper-tiny",
// "Systran/faster-whisper-tiny.en",
// "Systran/faster-whisper-base",
// "Systran/faster-whisper-base.en,",
// "Systran/faster-whisper-small",
// "Systran/faster-whisper-small.en",
// "Systran/faster-whisper-medium",
// "Systran/faster-whisper-medium.en",
// "Systran/faster-whisper-large-v1",
// "Systran/faster-whisper-large-v2",
// "Systran/faster-whisper-large-v3",

lenses: "asr-faster-whisper": {
  spawn: {}
  spawn: {
    environment: {
      MODEL_REPO: string
    }

    resourcedirs: {
      tiny: {
        id: "huggingface:model:Systran/faster-whisper-tiny:d90ca5fe260221311c53c58e660288d3deb8d356"
      }
      "large-v2": {
        id: "huggingface:model:Systran/faster-whisper-large-v2:f0fe81560cb8b68660e564f55dd99207059c092e"
      }
      // "large-v3": {
      //   id: "huggingface:model:Systran/faster-whisper-large-v3:edaa852ec7e145841d8ffdb056a99866b5f0a478"
      // }
    }

    parameters: _

    // HACK need this let in order for cue to properly resolve "parameters" field
    let p = parameters
    if p.cuda_memory_total.resource.quantity > 0 {
      environment: {
        MODEL_REPO: "/res/large-v2/huggingface/local"
        MODEL_DEVICE: "cuda"
        MODEL_COMPUTE_TYPE: "float16"
        CUDA_DEVICE_ORDER: "PCI_BUS_ID"
        CUDA_VISIBLE_DEVICES: "0"
      }
    }
    if p.cuda_memory_total.resource.quantity == 0 {
      environment: {
        MODEL_REPO: "/res/tiny/huggingface/local"
        MODEL_DEVICE: "cpu"
        MODEL_COMPUTE_TYPE: "int8"
      }
    }
  }

  calls: [
    {
      request: {
        url: path: "/v1/transcribe"
        // headers: "Content-Type": "application/json"
        body: asr.#Request
        body: audio_data !: bytes | string
      }
      response: {
        // headers: "Content-Type": "application/json"
        // body: asr.#Response
      }
    }
  ]
}

