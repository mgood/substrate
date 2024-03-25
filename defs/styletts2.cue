package defs

enable: "styletts2": true

live_edit: "styletts2": bool

imagespecs: "styletts2": {}

services: "styletts2": {
  spawn: {
    resourcedirs: {
      model: {
        id: "huggingface:model:yl4579/StyleTTS2-LibriTTS:d2ca3f14cf019cd2da653c74564e04f8e1f5c5ab"
      }
    }

    environment: {
      MODEL: "/res/model/huggingface/local"
    }

    command: [
      "--log-level", "debug",
      "--port", environment.PORT,
      "--reload", "--reload-dir", "/app",
      "--access-log",
    ]

    if live_edit["styletts2"] {
      mounts: [
        { source: "\(#var.host_source_directory)/images/styletts2", destination: "/app/", mode: "ro" },
      ]
    }
  }
}
