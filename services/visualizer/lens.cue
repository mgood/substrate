package lens

name: "visualizer"

spawn: schema: data: type: "space"

activities: {
  previewFiles: {
    activity: "system:preview:space"
    request: interactive: true
    request: path: "/"
    priority: 1
  }
}
