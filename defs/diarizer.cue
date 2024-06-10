package defs

import (
  "encoding/hex"
  cryptosha256 "crypto/sha256"
)

enable: "diarizer": true

imagespecs: "diarizer": {}

services: "diarizer": {
  spawn: {
    environment: {
      CUDA_DEVICE_ORDER: "PCI_BUS_ID"
      MODEL: "/app/speaker-diarization.yaml"
      PYANNOTE_CACHE: "/model-cache"
      PORT: string
    }
    resourcedirs: {
      // diarization: {
      //   id: "huggingface:model:pyannote/speaker-diarization-3.1:84fd25912480287da0247647c3d2b4853cb3ee5d"
      //   sha256: hex.Encode(cryptosha256.Sum256(id))
      //   #cache_key: "models--pyannote--speaker-diarization-3.1"
      // }
      segmentation: {
        id: "huggingface:model:pyannote/segmentation-3.0:e66f3d3b9eb0873085418a7b813d3b369bf160bb"
        sha256: hex.Encode(cryptosha256.Sum256(id))
        #cache_key: "models--pyannote--segmentation-3.0"
      }
      embedding: {
        id: "huggingface:model:pyannote/wespeaker-voxceleb-resnet34-LM:837717ddb9ff5507820346191109dc79c958d614"
        sha256: hex.Encode(cryptosha256.Sum256(id))
        #cache_key: "models--pyannote--wespeaker-voxceleb-resnet34-LM"
      }
    }
    mounts: [
      for _, rsc in resourcedirs {
        { source: "\(#var.build_resourcedirs_root)/\(rsc.sha256)/huggingface/cache/\(rsc.#cache_key)", destination: "\(environment.PYANNOTE_CACHE)/\(rsc.#cache_key)", mode: "ro" },
      }
    ]
  }
}
