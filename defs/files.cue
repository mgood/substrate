package defs

enable: "files": false

imagespecs: "files": {}

services: "files": {
  spawn: {}
  spawn: schema: data: type: "space"

  activities: {
    // previewFiles: {
    //   activity: "system:preview:space"
    //   request: interactive: true
    //   request: path: "/"
    //   priority: 1
    // }
    previewJupyverse: {
      activity: "system:preview:activity:jupyverse"
      request: interactive: true
      request: path: "/nbpreview"
      priority: 10
    }
    previewJupyterLab: {
      activity: "system:preview:activity:jupyter-lab"
      request: interactive: true
      request: path: "/nbpreview"
      priority: 10
    }
    browseFiles: {
      activity: "user:open"
      label: "browse files..."
      request: interactive: true
      // image: (svg.#SVGImageData & {
      //   // Heroicon name: mini/folder-open
      //   #src: """
      //     <svg class="-ml-1 mr-2 h-5 w-5 {iconClass}" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
      //       <path d="M4.75 3A1.75 1.75 0 003 4.75v2.752l.104-.002h13.792c.035 0 .07 0 .104.002V6.75A1.75 1.75 0 0015.25 5h-3.836a.25.25 0 01-.177-.073L9.823 3.513A1.75 1.75 0 008.586 3H4.75zM3.104 9a1.75 1.75 0 00-1.673 2.265l1.385 4.5A1.75 1.75 0 004.488 17h11.023a1.75 1.75 0 001.673-1.235l1.384-4.5A1.75 1.75 0 0016.896 9H3.104z" />
      //     </svg>
      //   """
      // }).#out
    }
  }
}
