from .machine import (
    Machine,
    MachineException,
    ReadingThread,
)
from data.shot import Shot
import re
from tkinter import simpledialog

header_regex_string = "\d{8}\w{2}"
shots_regex_string = (
    "(?P<ring>\D)(?P<tenth>\d)(?P<teiler>\d+)(?P<x>[+-]\d+)(?P<y>[+-]\d+)"
)


class QRReadingThread(ReadingThread):
    def run(self):
        self._messages = []
        self.result = []

        def unmap_ring(instr: str) -> str:
            return (
                instr.replace("A", "10.")
                .replace("B", "9.")
                .replace("C", "8.")
                .replace("D", "7.")
                .replace("E", "6.")
                .replace("F", "5.")
                .replace("G", "4.")
                .replace("H", "3.")
                .replace("I", "2.")
                .replace("J", "1.")
            )

        y = self.machine.settings.filepath[4:8]
        m = self.machine.settings.filepath[2:4]
        d = self.machine.settings.filepath[0:2]
        datum = f"{d}.{m}.{y}"  # ignore for now (refactoring of settings required)
        self.type_of_target = self.machine.settings.filepath[8:10]
        str_shots = self.machine.settings.filepath[10:]
        for shot in re.finditer(shots_regex_string, str_shots):
            self.result.append(
                Shot(
                    ringe=float(unmap_ring(shot["ring"]) + shot["tenth"]),
                    teiler=float(shot["teiler"]),
                    x=float(shot["x"]),
                    y=float(shot["y"]),
                )
            )
            self._messages.append(self.result[-1])


class QR(Machine):
    def get_string(self) -> str:
        return "QR-Code aus Eingabe"

    def config(self):
        self.settings.filepath = simpledialog.askstring(
            title="QR-Code", prompt="QR-Code eingeben"
        )
        if not re.search(shots_regex_string, self.settings.filepath):
            raise MachineException("Format of header of QR-Code does not match")
        if not re.search(shots_regex_string, self.settings.filepath):
            raise MachineException(f"Format of entries of QR-Code does not match")

    def get_reading_thread(self):
        thr = QRReadingThread()
        thr.machine = self
        return thr

    @staticmethod
    def get_available():
        return [QR()]

    @property
    def needs_setting(self) -> list[str]:
        return [
            "name",
            "date",
        ]
