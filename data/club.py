from dataclasses import dataclass, field
import json
import os
from typing import Dict
from dataclasses_json import dataclass_json
import uuid


@dataclass_json
@dataclass
class Club:
    name: str
    id: str = ""


CLUB_DB_VERSION = None


@dataclass_json
@dataclass
class ClubDB:
    clubs: Dict[str, Club] = field(default_factory=dict)
    version = None

    def save(self, file="./db/clubs.json"):
        with open(file, "w") as json_file:
            json.dump(
                json.loads(self.to_json()),
                json_file,
                indent=2,
            )

    def load(file="./db/clubs.json"):
        if not os.path.exists(os.path.dirname(file)):
            os.mkdir(os.path.dirname(file))
        try:
            with open(file, "r") as json_file:
                db = ClubDB.from_json(json_file.read())
                if not db.version == CLUB_DB_VERSION:
                    if db.version == None:
                        # provide upgrade from version n-1
                        pass
        except Exception as e:
            print(e)
            print("Teams file not existing")
            db = ClubDB(version=CLUB_DB_VERSION)
            db.save(file)
        return db

    def add_club(self, club: Club) -> str:
        if not club.id:
            club.id = str(uuid.uuid4())
        if not club.id in self.clubs.keys():
            self.clubs[club.id] = club
        return club.id

    def _get_name(item: tuple[str, Club]):
        return item[1].name

    def __getitem__(self, key):
        return self.clubs[key]

    def __iter__(self):
        return iter(sorted(self.clubs.items(), key=ClubDB._get_name))
