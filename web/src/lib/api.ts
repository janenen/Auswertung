// Typed client for the Go API (internal/server).

export interface Shot {
  ringe: number
  teiler: number
  x: number
  y: number
}

export interface User {
  id: string
  name: string
  birthday: string
}

export interface UserSummary extends User {
  matchCount: number
}

export interface Named {
  id: string
  name: string
}

export interface Competition {
  id: string
  name: string
  date: string
  count: number
  shots_per_target: number
  type_of_target: string
  decimal: boolean
  active: boolean
  modus: string
  /** ID of the league pairing, empty for other competitions. */
  league: string
}

export interface CompetitionSummary extends Competition {
  entryCount: number
}

export interface League {
  id: string
  discipline: string
  league: string
  group: string
  home_club: string
  home_team: string
  guest_club: string
  guest_team: string
  homeClubName: string
  homeTeamName: string
  guestClubName: string
  guestTeamName: string
}

export interface Entry {
  matchId: string
  shooterId: string
  shooter: string
  clubId: string
  club: string
  teamId: string
  team: string
  date: string
  shots: number
  active: boolean
  value: string
  summeGanz: number
  summe: number
  series: string[]
}

export interface CompetitionView {
  competition: Competition
  league: League | null
  entries: Entry[]
}

export interface Results extends CompetitionView {
  ranked: Entry[]
  home: Entry[]
  guest: Entry[]
  /** League scoring of home against guest (RSB-LO 7.2, 7.3). */
  score?: LeagueResult
}

export interface LeaguePairing {
  place: number
  /** Indexes into home/guest, -1 if the team has no shooter on this place. */
  home: number
  guest: number
  homePoints: number
  guestPoints: number
}

export interface LeagueResult {
  homePlaces: number[]
  guestPlaces: number[]
  pairings: LeaguePairing[]
  homePoints: number
  guestPoints: number
  /** Both teams have at least 3 final results; team points are awarded. */
  complete: boolean
  homeTeamPoints: number
  guestTeamPoints: number
}

export interface Stats {
  count: number
  summe: number
  summeGanz: number
  best: Shot
  ablageRL: number
  ablageHT: number
  rings: number[] // 10 down to 0
}

export interface MatchView {
  match: {
    id: string
    shooter: string
    type_of_target: string
    date: string
    shots: Shot[]
    active: boolean
    club: string
    team: string
    /** Empty for a quick analysis. */
    competition: string
  }
  shooter: string
  club: string
  team: string
  total: Stats
  series: Stats[]
}

export interface TargetGeometry {
  radiusTen: number
  radiusInnerTen: number
  ringIncrement: number
  radiusBlack: number
  radiusCalibre: number
}

export interface Meta {
  targets: Record<string, TargetGeometry>
  targetTypes: string[]
  modi: string[]
  leagueModus: string
  leagues: string[]
  /** Simulation mode (-simulator): simulated machine and league results. */
  simulator: boolean
}

export interface Machine {
  id: string
  name: string
  input: '' | 'file' | 'text'
  accept: string
}

export interface QuickSettings {
  name: string
  count: number
  shots_per_target: number
  type_of_target: string
}

export interface ReadingEvent {
  type: 'status' | 'shot' | 'error' | 'done'
  message?: string
  shot?: Shot
  targetType?: string
  matchId?: string
}

export interface BoardTile {
  key: string
  shooter: string
  targetType: string
  shots: Shot[]
  result: string
  series: string[]
  live: boolean
}

export interface BoardCompetition {
  id: string
  name: string
  count: number
  tiles: BoardTile[]
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch('/api' + path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    let message = res.statusText
    try {
      message = (await res.json()).error ?? message
    } catch {
      // keep status text
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

const get = <T>(path: string) => request<T>('GET', path)

export const api = {
  meta: () => get<Meta>('/meta'),

  users: () => get<UserSummary[]>('/users'),
  saveUser: (u: Partial<User>) =>
    u.id ? request<User>('PUT', `/users/${u.id}`, u) : request<User>('POST', '/users', u),
  clubs: () => get<Named[]>('/clubs'),
  saveClub: (c: Partial<Named>) =>
    c.id ? request<Named>('PUT', `/clubs/${c.id}`, c) : request<Named>('POST', '/clubs', c),
  teams: () => get<Named[]>('/teams'),
  saveTeam: (t: Partial<Named>) =>
    t.id ? request<Named>('PUT', `/teams/${t.id}`, t) : request<Named>('POST', '/teams', t),

  competitions: (active = true) => get<CompetitionSummary[]>(`/competitions?active=${active}`),
  competition: (id: string) => get<CompetitionView>(`/competitions/${id}`),
  results: (id: string) => get<Results>(`/competitions/${id}/results`),
  createCompetition: (c: Partial<Competition> & { leagueData?: Partial<League> }) =>
    request<Competition>('POST', '/competitions', c),
  setActive: (id: string, active: boolean) =>
    request<void>('PATCH', `/competitions/${id}`, { active }),
  deleteCompetition: (id: string) => request<void>('DELETE', `/competitions/${id}`),
  simulateLeague: (id: string) => request<{ matchIds: string[] }>('POST', `/competitions/${id}/simulate`),

  match: (id: string) => get<MatchView>(`/matches/${id}`),
  board: () => get<BoardCompetition[]>('/board'),

  machines: () => get<Machine[]>('/machines'),
  startReading: (r: {
    machineId: string
    competitionId?: string
    quick?: QuickSettings
    matchId?: string
    userId?: string
    clubId?: string
    teamId?: string
    data?: string // base64
  }) => request<{ id: string }>('POST', '/readings', r),
  stopReading: (id: string, keep: boolean) => request<void>('POST', `/readings/${id}/stop`, { keep }),

  shutdown: () => request<void>('POST', '/shutdown'),
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(binary)
}
