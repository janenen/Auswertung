from abc import ABC, abstractmethod
from dataclasses import dataclass
from dataclasses_json import dataclass_json
from threading import Thread
from data.shot import Shot

# ToDo: Add Disag


@dataclass_json
@dataclass
class MachineSettings:
    count: int | None
    shots_per_target: int | None
    type_of_target: str | None
    filepath: str | None


class MachineException(Exception):
    def __init__(self, message):
        self.message = message
        super().__init__(self.message)


class ReadingNotAvailable(Exception):
    pass


@dataclass_json
@dataclass
class Machine(ABC):
    settings: MachineSettings = None

    @abstractmethod
    def config(self, rest=None): ...

    @abstractmethod
    def get_string(self) -> str: ...

    @abstractmethod
    def get_reading_thread(self) -> "ReadingThread": ...

    @staticmethod
    @abstractmethod
    def get_available() -> list["Machine"]: ...

    @property
    @abstractmethod
    def needs_setting(self) -> list[str]: ...

    def __str__(self) -> str:
        return self.get_string()


class ReadingThread(Thread):
    _messages = []
    machine: Machine
    result = []
    type_of_target = None
    shutdown = False

    def get_reading(self):
        try:
            return self._messages.pop(0), self.type_of_target  # Workaround fix later
        except IndexError:
            raise ReadingNotAvailable

    def is_finished(self):
        return not self.is_alive() and not self._messages

    def get_result(self) -> list[Shot]:
        if self.is_finished():
            return self.result
