package dev

enable: "visualizer": false

imagespecs: "visualizer": {}

"lenses": "visualizer": {
  spawn: {}
  spawn: schema: data: type: "space"

  activities: {
    previewFiles: {
      activity: "system:preview:space"
      request: interactive: true
      request: path: "/"
      priority: 1
    }
  }
}