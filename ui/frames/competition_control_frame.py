from tkinter import ttk
from tkinter import messagebox
from idlelib.tooltip import Hovertip
from .default_frame import DefaultFrame
from ..windows.visualisation_window import Visualisation


class CompetitionControlFrame(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)

        # field options
        options = {"padx": 5, "pady": 0}
        self.columnconfigure(0, weight=1)
        # Wettkampfsteuerung
        self.general_competition_labelframe = ttk.LabelFrame(
            self, text="Wettkampfsteuerung"
        )
        self.general_competition_labelframe.columnconfigure(1, weight=1)
        self.add_competition_button = ttk.Button(
            self.general_competition_labelframe,
            text="Wettkampf hinzufügen",
            command=self.add_competition,
        )
        self.add_competition_button.grid(row=0, column=1, sticky="e")
        Hovertip(self.add_competition_button, "Fügt einen neuen Wettkampf hinzu")

        self.general_competition_labelframe.grid(row=0, column=0, sticky="ew")

        # Aktueller Wettkampf
        self.current_competition_labelframe = ttk.LabelFrame(
            self, text="Aktueller Wettkampf"
        )
        self.current_competition_labelframe.columnconfigure(2, weight=1)
        ttk.Label(self.current_competition_labelframe, text="Name:").grid(
            row=0, column=0, sticky="e"
        )
        ttk.Label(self.current_competition_labelframe, text="Beiträge:").grid(
            row=1, column=0, sticky="e"
        )
        self.competition_name_label = ttk.Label(
            self.current_competition_labelframe, text="-"
        )
        self.competition_name_label.grid(row=0, column=1, sticky="w")
        self.competition_count_label = ttk.Label(
            self.current_competition_labelframe, text="0"
        )
        self.competition_count_label.grid(row=1, column=1, sticky="w")
        self.add_entry_button = ttk.Button(
            self.current_competition_labelframe,
            text="Beitrag hinzufügen",
            command=self.add_entry,
        )
        self.add_entry_button.grid(row=0, column=2, sticky="e")
        Hovertip(
            self.add_entry_button,
            "Dem aktuellen Wettkampf einen weitern Beitrag hinzufügen",
        )
        self.show_entries_button = ttk.Button(
            self.current_competition_labelframe,
            text="Teilnehmer anzeigen",
            command=self.show_entries,
        )
        self.show_entries_button.grid(row=1, column=2, sticky="e")
        self.show_entries_button["state"] = "disabled"
        Hovertip(self.show_entries_button, "Zeigt die vorhandenen Ergebnisse an")

        self.finish_competition_button = ttk.Button(
            self.current_competition_labelframe,
            text="Wettkampfergebnisse anzeigen",
            command=self.finish_competition,
        )
        self.finish_competition_button.grid(row=3, column=2, sticky="e")
        self.finish_competition_button["state"] = "disabled"
        Hovertip(
            self.finish_competition_button,
            "Zeigt die aktuellen Wettkampfergebnisse an\nHier kann ein Druckbarer Bericht erzegt oder der Wettkampf beendet werden",
        )
        self.current_competition_labelframe.grid(row=1, column=0, sticky="ew")

        # Allgemeines
        self.general_labelframe = ttk.LabelFrame(self, text="Allgemeines")
        self.general_labelframe.columnconfigure(1, weight=1)
        self.quick_analysis_button = ttk.Button(
            self.general_labelframe,
            text="Schnellauswertung",
            command=self.quick_analysis,
        )
        self.quick_analysis_button.grid(row=0, column=1, sticky="e")
        self.quick_analysis_button.configure(command=self.quick_analysis)
        Hovertip(
            self.quick_analysis_button,
            "Schnellauswertung\n Das Ergebnis wird keinem Wettkampf hinzugefügt",
        )
        self.show_visualisation_button = ttk.Button(
            self.general_labelframe,
            text="Visualisierung",
            command=self.show_visualisation,
        )
        self.show_visualisation_button.grid(row=1, column=1, sticky="e")
        Hovertip(self.show_visualisation_button, "Visualisierungsfenster öffnen")
        self.load_competition_button = ttk.Button(
            self.general_labelframe,
            text="Wettkampf laden",
            command=self.show_old_competitions,
        )
        self.load_competition_button.grid(row=2, column=1, sticky="e")
        Hovertip(
            self.load_competition_button,
            "Lädt einen gespeicherten Wettkampf",
        )
        self.quit_button = ttk.Button(
            self.general_labelframe, text="Programm beenden", command=self.quit
        )
        self.quit_button.grid(row=3, column=1, sticky="e")
        Hovertip(
            self.quit_button,
            "Programm beenden",
        )

        self.general_labelframe.grid(row=3, column=0, sticky="ew")

        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def show_old_competitions(self):
        self.next_step = "show old competitions"
        self.proceed()

    def add_competition(self):
        self.next_step = "add competition"
        self.proceed()

    def finish_competition(self):
        self.next_step = "finish competition"
        self.proceed()

    def add_entry(self):
        self.next_step = "add entry"
        self.proceed()

    def quick_analysis(self):
        self.next_step = "quick analysis"
        self.proceed()

    def quit(self):
        MsgBox = messagebox.askquestion(
            "Wirklich beenden?",
            "Sind Sie sicher?",
            icon="error",
        )
        if MsgBox == "yes":
            self.next_step = "quit"
            self.proceed()

    def show_entries(self):
        self.next_step = "show entries"
        self.proceed()

    def show_visualisation(self):
        Visualisation(self.parent)

    def reset(self):
        self.deactivate_ok_button()
        self.deactivate_back_button()
        self.parent.competitions_frame.competition_listbox.configure(state="normal")
        self.parent.competitions_frame.update_competitions()
        self.parent.reset()
        self.show_entries_button["state"] = "disabled"
        self.add_entry_button["state"] = "disabled"
        self.finish_competition_button["state"] = "disabled"
        self.competition_name_label.config(text="-")
        self.competition_count_label.config(text="-")
        self.current_match = None

        if not self.add_to_current_competition:
            self.competition = None
            self.add_to_current_competition = True

        if self.competitions:
            if self.competition:
                self.add_entry_button["state"] = "normal"
                self.competition_name_label.config(text=self.competition.name)
                self.competition_count_label.config(text=len(self.competition.entries))
                self.finish_competition_button["state"] = "normal"
                if self.competition.entries:
                    self.show_entries_button["state"] = "normal"
