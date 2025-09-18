from dataclasses import dataclass, field
import json
import os
import uuid
import configparser
from dataclasses_json import dataclass_json
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .competition import Competition
    from .match import Match


@dataclass_json
@dataclass
class User:
    name: str
    birthday: str = ""
    niceness: int = 0
    id: str = ""
    matches: list[str] = field(default_factory=list)
    competitions: list[str] = field(default_factory=list)

    def add_match(self, match: "Match"):
        if not match.id in self.matches:
            self.matches.append(match.id)

    def add_competition(self, competition: "Competition"):
        if competition.id == "":
            raise Exception()
        if not competition.id in self.competitions:
            self.competitions.append(competition.id)


USER_DB_VERSION = 1


@dataclass_json
@dataclass
class UserDB:
    users: dict[str, User] = field(default_factory=dict)
    version: int | None = None

    def save(self, file="./db/users.json"):
        if not os.path.exists(os.path.dirname(file)):
            os.mkdir(os.path.dirname(file))
        with open(file, "w") as json_file:
            json_file.write(json.dumps(json.loads(self.to_json()), indent=2))

    def load(file="./db/users.json"):
        if os.path.exists(file):
            db = None
            try:
                with open(file, "r") as json_file:
                    db = UserDB.from_json(json_file.read())
            except:
                pass
            if db:
                if db.version == USER_DB_VERSION:
                    return db
                else:
                    if db.version == None:
                        pass  # cannot happen
                    elif db.version == 1:
                        # provide upgrade from version n-1
                        pass
            else:
                db = UserDB.convert_to_v1(file, UserDB(version=USER_DB_VERSION))
                db.save()
                return UserDB.load()
        elif os.path.exists("./schuetzen.ini"):
            print("old config found")
            db = UserDB(version=USER_DB_VERSION)
            userconfig = configparser.ConfigParser()
            userconfig.read("./schuetzen.ini")

            for section in userconfig.sections():
                if not (section == "Neu" or section == "NeuerSchütze"):
                    name = userconfig.get(section, "Name")
                    niceness = userconfig.getint(section, "niceness", fallback=0)
                    db.add_user(name=name, niceness=niceness)
            db.save(file)
            os.remove("./schuetzen.ini")
        else:
            print("User file not existing")
            db = UserDB(version=USER_DB_VERSION)
            db.save(file)
        return db

    def convert_to_v1(file, db):
        with open(file, "r") as oldfile:
            users = json.load(oldfile)
        for key in users["users"].keys():
            db.add_user(
                user=User(
                    name=users["users"][key]["shooter"]["name"],
                    birthday=users["users"][key]["shooter"]["birthday"],
                    niceness=users["users"][key]["niceness"],
                    id=users["users"][key]["id"],
                    matches=users["users"][key]["matches"],
                    competitions=users["users"][key]["competitions"],
                )
            )
        db.version = 1
        return db

    def add_user(self, user: User) -> str:
        if not user.id:
            user.id = str(uuid.uuid4())
        if not user.id in self.users.keys():
            self.users[user.id] = user
        return user.id

    def _get_niceness(item: tuple[str, User]):
        return item[1].niceness

    def __getitem__(self, key):
        return self.users[key]

    def __iter__(self):
        return iter(sorted(self.users.items(), key=UserDB._get_niceness, reverse=True))
