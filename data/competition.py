from dataclasses import dataclass
from datetime import date

from dataclasses_json import dataclass_json

from data.shot import Shot
from .match import Match, MatchSettings
from .shooter import Shooter
from machines.machine import Machine


# @dataclass_json
# @dataclass
class CompetitionSettings:
    name: str
    date: str
    count: int
    shots_per_target: int
    type_of_target: int
    decimal: bool


class Competition:
    settings: CompetitionSettings
    source: Machine
    entries: list[Match] = []
    _current_match: Match

    def __init__(self, settings):
        self.settings = settings

    def add_match(self, shooter: Shooter, shots: list[Shot]):
        new_match = Match(
            MatchSettings(
                competition=self.settings.name,
                decimal=self.settings.decimal,
                count=self.settings.count,
                date=date.today().strftime("%d.%m.%Y"),
                shooter=shooter,
                type_of_target=self.settings.type_of_target,
            )
        )
        new_match.shots = shots
        self.entries.append(new_match)
        self._current_match = self.entries[-1]

    @property
    def current_match(self):
        return self._current_match

    def get_sorted_results(self):
        return self.entries
        # add sort function later
