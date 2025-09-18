import datetime
import tkinter as tk
import time
from tkinter import messagebox
from tkinter import ttk
from data.match import Match
from machines.machine import MachineException, ReadingNotAvailable, ReadingThread
from ui.utils.target import TargetCanvas
from .default_frame import DefaultFrame


class ReadingFrame(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)
        self.reader: ReadingThread = None
        self.stopped = False
        # field options
        options = {"padx": 5, "pady": 0}
        self.statusbox = tk.Listbox(self, state="normal")
        self.statusbox.grid(column=0, row=0, sticky="nesw")

        self.canvsize = 200
        self.canvas = TargetCanvas(self, self.canvsize)
        self.canvas.grid(column=1, row=0, sticky="nse")
        self.stopbutton = ttk.Button(self, text="Stop", command=self.stop_reading)
        self.stopbutton.grid(column=1, row=1, padx=5, pady=5, sticky="nse")

        self.columnconfigure(0, weight=1)
        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def stop_reading(self):
        if self.reader:
            self.reader.shutdown = True
        self.stopped = True

    def actionStart(self):
        machine = self.source
        self.statusbox.insert("end", "Port wird geöffnet...")
        self.container.update()
        self.statusbox.insert("end", "Einstellungen werden übergeben...")
        self.container.update()
        try:
            if self.current_match:
                machine.config(rest=self.competition.count - self.current_match.anzahl)
            else:
                machine.config()  # config for the rest of series or default
        except MachineException as e:
            self.statusbox.insert("end", e.message)
            self.container.update()
            self.activate_back_button()
            return

        self.statusbox.insert("end", "Scheiben eingeben!")
        self.container.update()
        self.reader = machine.get_reading_thread()
        self.reader.start()
        first = True if not self.current_match else False
        while not self.reader.is_finished():
            self.container.update()
            try:
                shot, type_of_target = self.reader.get_reading()
            except ReadingNotAvailable:
                time.sleep(0.5)
                continue
            if not type_of_target == self.competition.type_of_target:
                self.reader.shutdown = True
                self.statusbox.insert("end", "Falscher Scheibentyp")
                self.container.update()
                messagebox.showerror("Falscher Scheibentyp", "Falscher Scheibentyp")
                self.recede()
                return
            self.type_of_target = type_of_target
            if first:
                first = False
                self.canvas.draw_target(self.type_of_target)
            self.canvas.draw_single_shot(shot, self.type_of_target)
        if self.reader.shutdown and not self.stopped:
            return
        if len(self.reader.get_result()) == 0:
            self.recede()
            return
        self.stopped = False
        self.statusbox.insert("end", "Einlesen abgeschlossen")
        self.container.update()
        if not self.current_match:
            self.current_match = Match(
                date=datetime.date.today().strftime("%d.%m.%Y"),
                shooter=self.user.id,
                type_of_target=self.type_of_target,
                shots=self.reader.get_result(),
                club=self.club.id if self.club else "",
                team=self.team.id if self.team else "",
            )
            self.matches.add_match(self.current_match)
            self.user.add_match(self.current_match)
            if self.add_to_current_competition:
                self.user.niceness += 1
                self.user.add_competition(self.competition)
                self.current_match.add_competition(self.competition)
                self.competition.add_match(self.current_match)
            self.users.save()
        else:
            self.current_match.shots.extend(self.reader.get_result())
        if len(self.current_match.shots) >= self.competition.count:
            self.current_match.active = False
        else:
            self.current_match.active = True
        self.matches.save()
        self.competitions.save()

        self.activate_back_button()
        self.activate_ok_button()
        self.proceed()

    def clear_target(self):
        self.canvas.delete("ring")
        self.canvas.delete("shot")

    def reset(self):
        if self.current_match:
            self.type_of_target = self.current_match.type_of_target
            self.canvas.draw_target(self.type_of_target)
            for shot in self.current_match.shots:
                self.canvas.draw_single_shot(shot, self.type_of_target)
        else:
            self.canvas.clear()

        self.activate_back_button()
        self.deactivate_ok_button()
        self.statusbox.delete("0", "end")
        self.statusbox.insert("end", "Bereit zum Einlesen")
        self.actionStart()
