package dev

#var: {
  host_docker_socket: string
  root_source_directory: string
}

enable: "openvscode": false

imagespecs: "openvscode": {
  build: {
    args: {
      RELEASE_TAG: "openvscode-server-v1.84.2"
    }
  }
}

"daemons": "openvscode": {
  environment: {
    #workspace: string | *"/home/workspace"
    #docker_socket: "/var/run/docker.sock"
    PORT: "3000" // HACK this is actually hardcoded internally in the image already...
    DOCKER_HOST: "unix://\(#docker_socket)"
  }

  command: ["--default-folder=\(environment.#workspace)", "--default-workspace=\(environment.#workspace)"]

  mounts: [
    {source: #var.root_source_directory, destination: environment.#workspace},
    {source: #var.host_docker_socket, destination: environment.#docker_socket},
  ]

  #systemd_units: {
    "openvscode.container": {
      Unit: {
        Requires: ["podman.socket"]
        After: ["podman.socket"]
      }
      Install: {
        WantedBy: ["multi-user.target", "default.target"]
      }
      Container: {
        AddDevice: ["/dev/kvm", "/dev/fuse"]
        SecurityLabelDisable: true
        PublishPort: [
          // To make localhost forwarding work (e.g. qemu, publish on the same port)
          "\(environment.PORT):\(environment.PORT)",
        ]
      }
    }
  }
}