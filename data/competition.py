from dataclasses import dataclass, field
import json
import os
import uuid
from dataclasses_json import dataclass_json
from .match import Match


SORTING_FUNCTION = {
    "Bestes Ergebnis": {"key": lambda x: x.get_result(), "reverse": True},
    "Bestes Ergebnis Zehntel": {"key": lambda x: x.get_result(True), "reverse": True},
    "Bester Teiler": {"key": lambda x: x.best.teiler, "reverse": False},
    "Liga des RSB (Kreis/Bezirk/Landesliga)": {},
    "Blödsinn: Anzahl 10er": {"key": lambda x: x.countRing(10), "reverse": True},
}


@dataclass_json
@dataclass
class Competition:
    name: str
    date: str
    count: int
    shots_per_target: int
    type_of_target: str
    decimal: bool
    active: bool = True
    modus: str = "Bestes Ergebnis"
    entries: list[str] = field(default_factory=list)
    id: str = ""
    league: str = ""

    def add_match(self, match: Match):
        if not match.id in self.entries:
            self.entries.append(match.id)


COMPETITION_DB_VERSION = None


@dataclass_json
@dataclass
class CompetitionDB:
    competitions: dict[str, Competition] = field(default_factory=dict)
    version: int | None = None

    def save(self, file="./db/competitions.json"):
        with open(file, "w") as json_file:
            json_file.write(json.dumps(json.loads(self.to_json()), indent=2))

    def load(file="./db/competitions.json"):
        if not os.path.exists(os.path.dirname(file)):
            os.mkdir(os.path.dirname(file))
        try:
            with open(file, "r") as json_file:
                db = CompetitionDB.from_json(json_file.read())
                if not db.version == COMPETITION_DB_VERSION:
                    if db.version == None:
                        # provide upgrade from version n-1
                        pass
        except Exception as e:
            print(e)
            print("Matches file not existing")
            db = CompetitionDB(version=COMPETITION_DB_VERSION)
            db.save(file)
        return db

    def add_competition(self, competition: Competition) -> str:
        if not competition.id:
            competition.id = str(uuid.uuid4())
        if not competition.id in self.competitions.keys():
            self.competitions[competition.id] = competition
        return competition.id

    def get_active_competitions(self) -> list[Competition]:
        returnlist = []
        for comp in self.competitions.items():
            if comp[1].active:
                returnlist.append(comp[1])
        return returnlist

    def get_inactive_competitions(self) -> list[Competition]:
        returnlist = []
        for comp in self.competitions.items():
            if not comp[1].active:
                returnlist.append(comp[1])
        return returnlist

    def remove(self, key):
        del self.competitions[key]

    def __getitem__(self, key):
        return self.competitions[key]

    def __iter__(self):
        return iter(self.competitions.items())
