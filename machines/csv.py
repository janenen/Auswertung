from tkinter import filedialog
from .machine import (
    VirtualMachine,
    MachineException,
    ReadingThread,
)
from data.shot import Shot
import re, os

header_regex_string = "(?P<date>\\d{2}.\\d{2}.\\d{4});(?P<count>\\d+);(?:b')?(?P<type_of_target>\\w{2})'?\\n"
shot_regex_string = "(?P<value>\\d*.\\d);(?P<distance>\\d*.\\d*);(?P<x>-?\\d*.\\d*);(?P<y>-?\\d*.\\d*)\\n"


class CSVReadingThread(ReadingThread):
    def run(self):
        self._messages = []
        self.result = []
        with open(self.machine.settings.filepath, "r") as infile:
            line = infile.readline()
            groupdict = re.match(header_regex_string, line).groupdict()
            datum = groupdict[
                "date"
            ]  # ignore for now (refactoring of settings required)
            count = int(groupdict["count"])
            self.type_of_target = groupdict["type_of_target"]

            for line in infile:
                line = line.replace(",", ".")
                groupdict = re.match(shot_regex_string, line).groupdict()
                self.result.append(
                    Shot(
                        ringe=float(groupdict["value"]),
                        teiler=float(groupdict["distance"]),
                        x=float(groupdict["x"]),
                        y=float(groupdict["y"]),
                    )
                )
                self._messages.append(self.result[-1])


class CSV(VirtualMachine):
    def get_string(self) -> str:
        return "CSV aus Datei"

    def config(self, rest=None):
        self.settings.filepath = filedialog.askopenfilename(
            initialdir="./",
            filetypes=[("CSV Dateien", "*.csv")],
        )
        if not os.path.isfile(self.settings.filepath):
            raise MachineException(
                f"File {self.settings.filepath} does not exist or is not a file"
            )
        with open(self.settings.filepath, "r") as infile:
            line = infile.readline()
            if not re.match(header_regex_string, line):
                raise MachineException(
                    f"Format of header of File {self.settings.filepath} does not match"
                )
            line = infile.readline()
            if not re.match(shot_regex_string, line):
                raise MachineException(
                    f"Format of first entry of File {self.settings.filepath} does not match"
                )

    def get_reading_thread(self):
        thr = CSVReadingThread()
        thr.machine = self
        return thr

    @property
    def needs_setting(self) -> list[str]:
        return [
            "name",
            "date",
        ]
