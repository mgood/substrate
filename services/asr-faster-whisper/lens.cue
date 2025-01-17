package lens

name: "asr-faster-whisper"

#build: {
  dockerfile: "./services/asr-faster-whisper/Dockerfile"
}

spawn: jamsocket: env: {
  MODEL_SIZE: "large-v2"
  MODEL_DEVICE: "cuda"
  MODEL_COMPUTE_TYPE: "float16"
  TORCH_HOME: "/cache/torch"
  CUDA_DEVICE_ORDER: "PCI_BUS_ID"
  CUDA_VISIBLE_DEVICES: "0"
  HF_HOME: "/cache/huggingface"
}
