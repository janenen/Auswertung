import tkinter as tk
from tkinter import ttk
from .default_frame import DefaultFrame


class Competitions(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)
        self.competition_listbox = tk.Listbox(self, height=14, width=25)

        self.competition_listbox.grid(column=0, row=0, sticky="nesw")
        # add padding to the frame and show it
        self.grid(column=0, row=0, padx=5, pady=5, sticky="nsew")

    def select(self, event):
        selection = self.competition_listbox.curselection()
        if len(selection) > 0:
            n = selection[0]
        else:
            return
        if self.showing == "competitions":
            self.competition = self.active_competitions[n]
            if self.competition.league:
                self.league = self.leagues[self.competition.league]
            else:
                self.league = None
            if self.competition.entries:
                self.current_match = self.matches[self.competition.entries[-1]]
        elif self.showing == "entries":
            self.current_match = self.matches[self.competition.entries[n]]
        self.parent.frame.reset()

    def update_competitions(self):
        self.showing = "competitions"
        self.competition_listbox.delete("0", "end")
        for comp in self.competitions.get_active_competitions():
            self.competition_listbox.insert("end", comp.name)
        self.competition_listbox.bind("<<ListboxSelect>>", self.select)

    def update_entries(self):
        self.showing = "entries"
        self.competition_listbox.delete("0", "end")
        for entry in self.competition.entries:
            self.competition_listbox.insert("end", self.matches[entry].shooter.name)
        self.competition_listbox.bind("<<ListboxSelect>>", self.select)
