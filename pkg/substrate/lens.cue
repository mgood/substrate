package lens

import (
  containerspec "github.com/ajbouh/substrate/pkg/substrate:containerspec"
  blackboard_http_call "github.com/ajbouh/substrate/pkg/blackboard:http_call"
)

#ServiceDefSpawnParameter: {
  type: "space" | "spaces" | "string" | "resource"
  // if type == "spaces" {
  //   // Default attributes to be used if we use a collection
  //   collection: attributes: {[string]: _}
  // }

  if type == "space" {
    space: {
      uses_preview ?: string
      // is_read_only: bool
    }
  }
  if type == "spaces" {
    spaces: [...string]
  }
  if type == "resource" {
    resource: {
      unit: string
      quantity: number
    }
    value: =~"^([0-9]+)\(resource.unit)$"
  }

  value: string
  description ?: string
  optional ?: bool
}

#ActivityDefRequestSchema: {
  [string]: {
    type: "space" | "collection" | "file"
    body ?: [...[...string]] | [...string]  // If true, set body to it. If a string[], set those top-level JSON fields with it. If string[][], set those JSON field selections to it.
    path ?: true | string // If true, replace path with it. If a string, replace string in path with file
    query ?: string | [...string] // Name OR list of names of query parameter
    if type == "file" {
      default ?: string
    }
  }
}

#ActivityDefResponseSchema: {
  [name=string]: {
    type: "space" | "collection" | "file"
    from: "header" | *"body"
    if from == "body" {
      path: [...string] | *[name]
    }
    if from == "header" {
      path: [string]
    }
  }
}

#ActivityDef: {
  activity: "user:new-space" | "user:open" | "user:fork" | "user:collection:space" | "system:preview:space" | =~ "^system:preview:activity:[^:]+$"

  label ?: string
  description ?: string
  after ?: [...string]
  priority ?: int
  image ?: string

  request ?: {
    interactive ?: bool
    path ?: string
    method: string | *"GET"

    schema ?: #ActivityDefRequestSchema
  }

  response ?: {
    schema ?: #ActivityDefResponseSchema
  }
}

let cs = containerspec
let #ServiceDef = close({
  containerspec: cs
  // no mounts allowed for lenses
  containerspec: mounts: []

  spawn ?: {
    parameters: [string]: #ServiceDefSpawnParameter
    parameters: {
      cuda_memory_total: {
        type: "resource"
        resource: {unit: "MB", quantity: number}
      }

      cpu_memory_total: {
        type: "resource"
        resource: {unit: "MB", quantity: number}
      }
    }
    "image": containerspec.image
    "environment": containerspec.environment

    // for name, parameter in parameters {
    //   if parameter.type == "space" {
    //     if parameter.space.is_read_only {
    //       environment["JAMSOCKET_SPACE_\(name)_readonly"] = 1
    //     }
    //   }
    // }
  }

  calls ?: [...blackboard_http_call.#Call]

  activities ?: [string]: #ActivityDef
})

#ServiceDef
