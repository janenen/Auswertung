from dataclasses import dataclass
from dataclasses_json import dataclass_json
import math


@dataclass_json
@dataclass
class Shot:
    ringe: float
    teiler: float
    x: float
    y: float

    @property
    def ringe_ganz(self):
        return math.floor(self.ringe)

    def __lt__(self, other: "Shot"):
        return self.teiler > other.teiler
