from dataclasses import dataclass, field
import json
import os
import uuid

from dataclasses_json import dataclass_json


@dataclass_json
@dataclass
class RSBLeague:
    discipline: str
    league: str
    group: str
    home_club: str
    home_team: str
    guest_club: str
    guest_team: str
    id: str = ""


LEAGUE_DB_VERSION = None


@dataclass_json
@dataclass
class LeagueDB:
    leagues: dict[str, RSBLeague] = field(default_factory=dict)
    version: int | None = None

    def save(self, file="./db/leagues.json"):
        with open(file, "w") as json_file:
            json_file.write(json.dumps(json.loads(self.to_json()), indent=2))

    def load(file="./db/leagues.json"):
        if not os.path.exists(os.path.dirname(file)):
            os.mkdir(os.path.dirname(file))
        try:
            with open(file, "r") as json_file:
                db = LeagueDB.from_json(json_file.read())
                if not db.version == LEAGUE_DB_VERSION:
                    if db.version == None:
                        # provide upgrade from version n-1
                        pass
        except Exception as e:
            print(e)
            print("Leagues file not existing")
            db = LeagueDB(version=LEAGUE_DB_VERSION)
            db.save(file)
        return db

    def add_league(self, league: RSBLeague) -> str:
        if not league.id:
            league.id = str(uuid.uuid4())
        if not league.id in self.leagues.keys():
            self.leagues[league.id] = league
        return league.id

    def remove(self, key):
        del self.leagues[key]

    def __getitem__(self, key):
        return self.leagues[key]

    def __iter__(self):
        return iter(self.leagues.items())
