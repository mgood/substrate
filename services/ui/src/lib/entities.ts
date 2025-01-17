import { error } from '@sveltejs/kit'

import {
  urls,
  fetchJSON,
  processLenses,
  processLensSpecs,
  processSpaces,
  processEvents,
  processActivities,
} from '$lib/activities'

export interface EntityListTruncation {
  seeAllLabel: string
  seeAllHref: string
  limit: number
}

export interface BaseEntitySelection {
  label: string
  truncation?: EntityListTruncation
}

// TODO make these real
export type Collection = any
export type Space = any
export type Lens = any
export type Event = any
export type Activity = any

export interface CollectionSelection extends BaseEntitySelection {
  entityType: "collection"
  entities: Collection[]
}

export interface ActivitySelection extends BaseEntitySelection {
  entityType: "activity"
  entities: Activity[]
}

export interface SpaceSelection extends BaseEntitySelection {
  entityType: "space"
  entities: Space[]
}

export interface LensSelection extends BaseEntitySelection {
  entityType: "lens"
  entities: Lens[]
}

export interface EventSelection extends BaseEntitySelection {
  entityType: "event"
  entities: Event[]
}

export type EntitySelection = LensSelection | SpaceSelection | CollectionSelection | ActivitySelection | EventSelection

export async function load({ params, url, fetch }) {
  console.log({ params, url })

  const {
    owner,
    type,
    id,
  } = params

  const selections: EntitySelection[] = []

  if (owner) {
    switch (type) {
      case "collections": {
        if (id) {
          const spaces = await fetchJSON(fetch, urls.api.collectionSpaceMembership({ owner, name: id }))
          selections.push({
            entityType: "space",
            entities: processSpaces(spaces),
            label: `${owner}/${id} Spaces`,
          })
          const lenses = await fetchJSON(fetch, urls.api.collectionLensMembership({ owner, name: id }))
          selections.push({
            entityType: "lens",
            entities: processLensSpecs(lenses),
            label: `${owner}/${id} Lenses`,
          })
        } else {
          const collections = await fetchJSON(fetch, urls.api.collections({ owner }))
          selections.push({
            entityType: "collection",
            entities: collections,
            label: `${owner}'s Collections`,
          })
        }
        break
      }
      case "spaces": {
        const spaces = await fetchJSON(fetch, urls.api.spaces({ owner }))
        selections.push({
          entityType: "space",
          entities: processSpaces(spaces),
          label: `${owner}'s Spaces`,
        })
        break
      }
      case undefined: {
        const spaces = await fetchJSON(fetch, urls.api.spaces({ owner }))
        selections.push({
          entityType: "space",
          entities: processSpaces(spaces),
          label: `${owner}'s Spaces`,
          truncation: {
            limit: 5,
            seeAllHref: urls.ui.userSpaces({ user: owner }),
            seeAllLabel: `See all ${owner}'s spaces`,
          },
        })

        const collections = await fetchJSON(fetch, urls.api.collections({ owner }))
        selections.push({
          entityType: "collection",
          entities: collections,
          label: `${owner}'s Collections`,
          truncation: {
            limit: 5,
            seeAllHref: urls.ui.userCollections({ user: owner }),
            seeAllLabel: `See all ${owner}'s collections`,
          },
        })
        break
      }
      default:
        throw error(404)
    }
  } else {
    switch (type) {
      case "lenses": {
        const lenses = await fetchJSON(fetch, urls.api.lenses({}))
        selections.push({
          entityType: "lens",
          entities: processLenses(lenses),
          label: `Lenses`,
        })
        break
      }
      case "spaces": {
        const spaces = await fetchJSON(fetch, urls.api.spaces({}))
        selections.push({
          entityType: "space",
          entities: processSpaces(spaces),
          label: `Spaces`,
        })
        break
      }
      case "activities": {
        const activities = await fetchJSON(fetch, urls.api.activities({}))
        selections.push({
          entityType: "activity",
          // HACK we should exclude these "system" activities in a more principled way
          entities: processActivities(activities.filter(({ activityspec }) => !/^(screenshot|files|visualizer)\[/.test(activityspec))),
          label: `Activities`,
        })
        break
      }
      case "events": {
        const events = await fetchJSON(fetch, urls.api.events({}))
        selections.push({
          entityType: "event",
          entities: processEvents(events),
          label: `Activity Feed`,
        })
        break
      }
      default:
        throw error(404)
    }
  }

  console.log({ selections })

  return {
    owner,
    selections,
  }
}
