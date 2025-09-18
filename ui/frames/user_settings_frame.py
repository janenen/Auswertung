import tkinter as tk
from tkinter import ttk
from idlelib.tooltip import Hovertip
from data.user import User
from .default_frame import DefaultFrame


class UserSettingsFrame(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)
        # field options
        options = {"padx": 5, "pady": 0}
        # constant labels
        ttk.Label(self, text="Name:").grid(column=0, row=0, sticky="e", **options)
        ttk.Label(self, text="Geburtstag:").grid(column=0, row=1, sticky="e", **options)

        self.name = tk.StringVar()
        self.name_entry = ttk.Entry(self, textvariable=self.name)
        self.name_entry.grid(column=1, row=0, sticky="w", **options)
        Hovertip(self.name_entry, "Name des Schützen")

        self.birthday = tk.StringVar()
        self.birthday_entry = ttk.Entry(self, textvariable=self.birthday)
        self.birthday_entry.grid(column=1, row=1, sticky="w", **options)
        Hovertip(self.birthday_entry, "Geburtstag des Schützen")

        self.save = tk.BooleanVar()
        self.save_box = tk.Checkbutton(self)
        self.save_box["text"] = "Speichern"
        self.save_box["variable"] = self.save
        self.save_box.grid(column=1, row=3, sticky="w", **options)
        Hovertip(self.save_box, "Daten des Schützen und Einstellungen speichern")

        # add padding to the frame and show it
        self.columnconfigure(0, weight=1)
        self.columnconfigure(1, weight=1)
        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def reset(self):
        self.name.set("")
        self.birthday.set("")
        if self.user:  # user selected to edit
            self.name.set(self.user.name)
            self.birthday.set(self.user.birthday)
        self.save.set(False)
        self.activate_back_button()
        self.activate_ok_button()

    def parseInput(self):
        name = self.name.get()
        birthday = self.birthday.get()
        if name == "":
            return
        if self.user:
            self.users[self.user.shooter].name = name
            self.users[self.user.shooter].birthday = birthday
        else:
            self.user = User(
                name=name,
                birthday=birthday,
            )
            if self.save.get():
                self.users.add_user(self.user)
        if self.save.get():
            self.users.save()
        return True
