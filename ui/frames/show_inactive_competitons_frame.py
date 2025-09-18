import tkinter as tk
from tkinter import ttk
from idlelib.tooltip import Hovertip
from .default_frame import DefaultFrame


class ShowInactiveCompetitions(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)

        options = {"padx": 5, "pady": 0}
        self.scrollbar = tk.Scrollbar(self)
        self.competitionlistbox = tk.Listbox(self, height=10, width=50)

        self.competitionlistbox.bind("<<ListboxSelect>>", self.select)
        self.competitionlistbox.config(yscrollcommand=self.scrollbar.set)
        self.scrollbar.config(command=self.competitionlistbox.yview)
        self.scrollbar.grid(column=2, row=0, sticky="wens")
        self.competitionlistbox.grid(columnspan=2, column=0, row=0, sticky="ew")

        self.test_label = ttk.Label(self, text="")
        self.test_label.grid(columnspan=2, column=0, row=2, sticky="nesw", **options)

        self.activate_competition_button = ttk.Button(self, text="Wettkampf öffnen")
        self.activate_competition_button.grid(column=1, row=3, sticky="e", **options)
        Hovertip(self.activate_competition_button, "Den Wettkampf wieder öffnen")
        self.activate_competition_button.configure(command=self.activate)

        self.delete_button = ttk.Button(self, text="Löschen")
        self.delete_button.grid(column=2, row=3, sticky="e", **options)
        Hovertip(self.delete_button, "Den Wettkampf löschen")
        self.delete_button.configure(command=self.delete)
        self.columnconfigure(0, weight=1)

        # ToDo: Shortcut show results

        # add padding to the frame and show it
        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def select(self, event):
        selection = self.competitionlistbox.curselection()
        if len(selection) > 0:
            n = selection[0]
            if n < len(self.competitions.get_inactive_competitions()):
                self.selected_competition = (
                    self.competitions.get_inactive_competitions()[n]
                )
                self.test_label.config(text=self.selected_competition.name)
                self.activate_competition_button["state"] = "normal"
                self.delete_button["state"] = "normal"
            else:
                return

        self.activate_back_button()
        self.activate_ok_button()

    def activate(self):
        self.selected_competition.active = True
        self.competitions.save()
        self.active_competitions.append(self.selected_competition)
        self.parent.competitions_frame.competition_listbox.configure(state="normal")
        self.parent.competitions_frame.update_competitions()
        self.parent.competitions_frame.competition_listbox.configure(state="disabled")
        self.reset()

    def delete(self):
        pass

    def reset(self):
        self.test_label.config(text="Wettkampf auswählen")
        self.selected_competition = None

        self.competitionlistbox.delete("0", "end")

        for competition in self.competitions.get_inactive_competitions():
            self.competitionlistbox.insert(
                "end",
                f"{competition.name}, {competition.date}, {len(competition.entries)} Beiträge",
            )

        self.competitionlistbox.bind("<<ListboxSelect>>", self.select)

        self.delete_button["state"] = "disabled"
        self.activate_competition_button["state"] = "disabled"
        self.activate_back_button()
