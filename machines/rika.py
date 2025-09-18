from .machine import Machine, MachineException
import serial, time, re
from serial.tools.list_ports_common import ListPortInfo
from data.shot import Shot
from .machine import ReadingThread

NUL = b"\x00"
SOH = b"\x01"
EOT = b"\x04"
ACK = b"\x06"
_REPEAT = b"\x07"
BS = b"\x08"
FF = b"\x0c"
CR = b"\x0d"
SYN = b"\x16"
ESC = b"\x1b"
ABM = b"\xd0"
EINZEL = b"\xd1"
_10SER = b"\xd2"
GESAMT = b"\xd3"
_5SER = b"\xd4"
REST = b"\xd5"


class RikaReadingThread(ReadingThread):
    def run(self):
        first = True
        regex_string = "b'(?P<serial>\\d{8})\\\\r(?P<bus_address>\\d{3})\\\\r(?P<manual_code>\\d{8})\\\\r(?P<type_of_target>\\w{2})\\\\r(?P<distance_factor>\\d{2})\\\\r(?P<count>\\d{3})\\\\r(?P<value>\\d{3})\\\\r(?P<distance>\\d{5})\\\\r(?P<x>[+-]\\d{5})\\\\r(?P<y>[+-]\\d{5})\\\\r(?P<checksum>.*)'"
        regex = re.compile(regex_string)
        self._messages = []
        self.result = []
        with self.machine.connection as conn:
            while len(self.result) < self.machine.rest:
                if self.shutdown:
                    break
                conn.write(SYN)
                ans = conn.read(1)
                if ans == SOH:
                    groupdict = regex.match(str(conn.read(32 + 24 + 2))).groupdict()
                    if first:
                        self.type_of_target = groupdict["type_of_target"]
                        first = False
                    self.result.append(
                        Shot(
                            ringe=int(groupdict["value"]) / 10,
                            teiler=int(groupdict["distance"]) / 10,
                            x=int(groupdict["x"]) / 100,
                            y=int(groupdict["y"]) / 100,
                        )
                    )
                    self._messages.append(self.result[-1])
                    conn.write(FF)  # FF
                else:
                    time.sleep(0.2)
            conn.write(ABM)
            conn.read(1)


class Rika(Machine):
    def is_available(self):
        with self.connection as conn:
            conn.read()
            conn.write(EINZEL)  # d1 einzeltreffer
            ans = conn.read(3)
            conn.write(ABM)
            conn.read(1)
            return ans == b"200"

    def set_port(self, port):
        if type(port) == ListPortInfo:
            self.connection = serial.Serial(port.name, 9600, timeout=0.5)
        else:
            raise Exception(message="Not a serial port")

    def get_string(self) -> str:
        return f"Rika SAG-2 an {self._connection.name}"

    def config(self, rest=None):
        self.rest = rest if rest else self.settings.count
        with self.connection as conn:
            conn.read()  # flush buffer
            conn.write(EINZEL)  # d1 einzeltreffer
            ans = conn.read(3)
            if ans != b"200":
                raise MachineException("Rika antwortet nicht")
            # ESC S XXX CR
            conn.write(
                ESC
                + b"S"
                + bytes(
                    str(self.settings.count if not rest else rest).zfill(3), "utf-8"
                )
                + CR
            )
            ans = conn.read(1)
            if ans != ACK:
                conn.write(ABM)
                raise MachineException("Anzahl konnte nicht übernommen werden")
            # ESC U X CR
            conn.write(
                ESC
                + b"U"
                + bytes(str(self.settings.shots_per_target).zfill(1), "utf-8")
                + CR
            )
            ans = conn.read(1)
            if ans != ACK:
                conn.write(ABM)
                raise MachineException(
                    "Schuss pro Scheibe konnte nicht übernommen werden"
                )
            # ESC Z 3 CR
            conn.write(ESC + b"Z3" + CR)
            ans = conn.read(1)
            if ans != ACK:
                conn.write(ABM)
                raise MachineException("Teilerwertung konnte nicht übernommen werden")

    def get_reading_thread(self):
        thr = RikaReadingThread()
        thr.machine = self
        return thr

    @property
    def needs_setting(self) -> list[str]:
        return [
            "name",
            "date",
            "count",
            "shots_per_target",
        ]
