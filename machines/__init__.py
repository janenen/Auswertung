from .machine import Machine


def find_machines() -> list[Machine]:
    from .rika import Rika
    from .csv import CSV
    from .qsd import QSD
    from .qr import QR

    known_machines: list[Machine] = [
        Rika,
        CSV,
        QR,
        QSD,
    ]

    machines: list[Machine] = []
    for machine in known_machines:
        machines.extend(machine.get_available())
    return machines
