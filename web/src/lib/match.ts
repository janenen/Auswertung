// Loading a match with the context its title and scoring come from; shared
// by the match page and its print view.
import { api, type Competition, type MatchView, type Shot } from './api'

export interface MatchContext {
  view: MatchView
  competition: Competition | null
}

/** Loads a match and its competition (from the query or the match itself). */
export async function loadMatch(id: string, competitionId = ''): Promise<MatchContext> {
  const view = await api.match(id)
  const cid = competitionId || view.match.competition
  const competition = cid ? (await api.competition(cid)).competition : null
  return { view, competition }
}

/**
 * Title and scoring: the competition's, else the query's (title/decimal),
 * else the settings stored with a quick analysis.
 */
export function matchTitle(ctx: MatchContext, query: Record<string, string>) {
  const { competition, view } = ctx
  return {
    title: competition?.name ?? query.title ?? (view.match.title || 'Schnellauswertung'),
    decimal: competition ? competition.decimal : query.decimal ? query.decimal === 'true' : view.match.decimal,
  }
}

/** Sums of shots, shaped like Stats for result(). */
export function sums(shots: Shot[]) {
  return {
    summe: Math.round(shots.reduce((a, s) => a + s.ringe, 0) * 10) / 10,
    summeGanz: shots.reduce((a, s) => a + Math.floor(s.ringe), 0),
  }
}
