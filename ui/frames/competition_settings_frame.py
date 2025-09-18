import tkinter as tk
from tkinter import ttk
from tkinter.messagebox import showerror
from datetime import date
from idlelib.tooltip import Hovertip
from data.match import RADIUS_DICT
from data.competition import Competition, SORTING_FUNCTION
from .default_frame import DefaultFrame


class CompetitionSettingsFrame(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)
        # field options
        options = {"padx": 5, "pady": 0}

        # constant labels
        ttk.Label(self, text="Name:").grid(column=0, row=0, sticky="e", **options)
        ttk.Label(self, text="Datum:").grid(column=0, row=1, sticky="e", **options)
        ttk.Label(self, text="Schusszahl:").grid(column=0, row=2, sticky="e", **options)
        ttk.Label(self, text="Schuss pro Scheibe:").grid(
            column=0, row=3, sticky="e", **options
        )
        ttk.Label(self, text="Scheibentyp:").grid(
            column=0, row=4, sticky="e", **options
        )
        ttk.Label(self, text="Zehntelwertung:").grid(
            column=0, row=5, sticky="e", **options
        )
        ttk.Label(self, text="Modus:").grid(column=0, row=6, sticky="e", **options)

        self.name = tk.StringVar()
        self.name_entry = ttk.Entry(self, textvariable=self.name)
        self.name_entry.grid(column=1, row=0, sticky="w", **options)
        Hovertip(
            self.name_entry,
            "Name des Bewerbs, z.B.\nTraining\nLigawettkampf\nPreisschießen",
        )

        self.date = tk.StringVar()
        self.date_entry = ttk.Entry(self, textvariable=self.date)
        self.date_entry.grid(column=1, row=1, sticky="w", **options)
        Hovertip(self.date_entry, "Datum des Bewerbs")

        self.count = tk.StringVar()
        self.count_entry = ttk.Entry(self, textvariable=self.count)
        self.count_entry.grid(column=1, row=2, sticky="w", **options)
        Hovertip(
            self.count_entry,
            "Anzahl der zu wertenden Schüsse",
        )

        self.shots_per_target = tk.StringVar()
        self.shots_per_target_entry = ttk.Entry(
            self, textvariable=self.shots_per_target
        )
        self.shots_per_target_entry.grid(column=1, row=3, sticky="w", **options)
        Hovertip(
            self.shots_per_target_entry,
            "Anzahl der Schüsse pro Scheibe\nNur bei Maschinenauswertung erforderlich\nMinimum: 1\nMaximum: 5",
        )

        self.type_of_target = tk.StringVar()
        self.type_of_target_menu = tk.OptionMenu(
            self, self.type_of_target, *list(RADIUS_DICT.keys())
        )
        self.type_of_target_menu.grid(column=1, row=4, sticky="w", **options)
        Hovertip(
            self.type_of_target_menu,
            "Art der Scheibe / Disziplin",
        )

        self.mode = tk.StringVar()
        self.mode_menu = tk.OptionMenu(self, self.mode, *list(SORTING_FUNCTION.keys()))
        self.mode_menu.grid(column=1, row=6, sticky="w", **options)
        Hovertip(
            self.type_of_target_menu,
            "Art der Wertung",
        )

        # add padding to the frame and show it
        self.columnconfigure(0, weight=1)
        self.columnconfigure(1, weight=1)
        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def reset(self):
        self.name.set("Training")
        self.date.set(date.today().strftime("%d.%m.%Y"))
        self.count.set("40")
        self.shots_per_target.set("1")
        self.type_of_target.set("LG")
        self.mode.set("Bestes Ergebnis")

        self.activate_back_button()
        self.activate_ok_button()

    def parseInput(self):
        name = self.name.get()
        type_of_target = self.type_of_target.get()
        if not type_of_target in RADIUS_DICT.keys():
            showerror(
                title="Scheibentyp auswählen",
                message="Kein gültiger Scheibentyp ausgewählt",
            )
            return False
        date = self.date.get()

        try:
            count = int(self.count.get())
        except ValueError as error:
            showerror(title="Das ist keine Zahl", message=error)
            return False

        try:
            shots_per_target = int(self.shots_per_target.get())
        except ValueError as error:
            showerror(title="Das ist keine Zahl", message=error)
            return False
        mode = self.mode.get()
        if mode == "":
            showerror(
                title="Modus auswählen",
                message="Kein gültiger Modus ausgewählt",
            )
            return False
        decimal = True if mode == "Bestes Ergebnis Zehntel" else False
        self.competition = Competition(
            name=name,
            date=date,
            count=count,
            shots_per_target=shots_per_target,
            type_of_target=type_of_target,
            decimal=decimal,
            modus=mode,
        )

        if self.add_to_current_competition:
            self.competitions.add_competition(self.competition)
            self.competitions.save()
            self.active_competitions.append(self.competition)
        self.parent.competitions_frame.competition_listbox.configure(state="normal")
        self.parent.competitions_frame.update_competitions()
        self.parent.competitions_frame.competition_listbox.configure(state="disabled")
        return True
