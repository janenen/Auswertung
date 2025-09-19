from math import ceil
import os
import sys
from threading import Thread
import time
import tkinter as tk
from tkinter.font import Font
import tkinter.ttk as ttk
from typing import TYPE_CHECKING

from data.competition import Competition
from data.match import Match
from ui.utils.target import TargetCanvas

if TYPE_CHECKING:
    from ui.frames.control_frame import ControlFrame


class Visualisation(tk.Toplevel):
    def __init__(self, master: "ControlFrame" = None):
        super().__init__(master=master)
        self.parent = master
        self.title("Visualisierung")
        self.geometry("1130x800")
        try:
            # PyInstaller creates a temp folder and stores path in _MEIPASS
            base_path = sys._MEIPASS
        except Exception:
            base_path = os.path.abspath(".")
        self.iconbitmap(os.path.join(base_path, "ui", "logo.ico"))
        self.competition_frames: dict[str, CompetitionVisualisationFrame] = {}
        self.keep_updating = True
        self.columnconfigure(0, weight=1)
        self.rowconfigure(0, weight=1)
        Thread(target=self.update_thread_function).start()

    def update_frame(self):
        for competition in self.parent.active_competitions:
            if competition.entries:
                if not competition.id in self.competition_frames.keys():
                    self.competition_frames[competition.id] = (
                        CompetitionVisualisationFrame(self, competition)
                    )
            for frame in self.competition_frames.keys():
                if frame not in [comp.id for comp in self.parent.active_competitions]:
                    del self.competition_frames[frame]

    def update_thread_function(self):
        while self.keep_updating:
            self.update_frame()
            try:
                for id in self.competition_frames:
                    self.competition_frames[id].update_frame()
                    self.competition_frames[id].tkraise()
                    time.sleep(5)
            except tk.TclError as e:
                print(e)
                self.keep_updating = False


class CompetitionVisualisationFrame(ttk.Frame):
    def __init__(self, container, competition: Competition):
        super().__init__(container)
        self.container: Visualisation = container
        self.competition: Competition = competition
        self.frame_size = 0

        self.match_frames: dict[str, ResultVisualisationFrame] = {}
        label = ttk.Label(self, text=competition.name)

        label.grid(column=0, row=0)

        self.grid(row=0, column=0, sticky="nesw")

    def update_frame(self):
        combinations = []
        k = len(self.competition.entries)
        for i in range(k):
            combinations.append((i + 1, ceil(k / (i + 1))))
        width = self.winfo_width()
        height = self.winfo_height()
        optimal_combination = (1, 1)
        area = 0
        size = 0
        for combination in combinations:
            max_height = (height / 1.5) / combination[0]
            max_width = (width - 20) / combination[1]
            constrian = min(max_height, max_width)
            area0 = constrian * (constrian * 1.5) * k
            if area0 > area:
                optimal_combination = combination
                area = area0
                size = constrian
        m = optimal_combination[0]
        n = optimal_combination[1]
        if not size == self.frame_size:
            self.frame_size = size
            for frame in self.match_frames.items():
                frame[1].destroy()
            self.match_frames.clear()

        for match_id in self.competition.entries:
            if not match_id in self.match_frames.keys():
                self.match_frames[match_id] = ResultVisualisationFrame(
                    self, self.container.parent.matches[match_id]
                )
            else:
                self.match_frames[match_id].update_frame()
        for i, frame in enumerate(self.match_frames.items()):
            frame[1].grid(row=i // n, column=i % n, sticky="nesw")


class ResultVisualisationFrame(ttk.Frame):
    def __init__(self, container, match: Match):
        self.container: CompetitionVisualisationFrame = container
        super().__init__(container)
        self.width = self.container.frame_size

        self.match: Match = match
        self.number_font = Font(self, "Helvetica")
        self.name_font = Font(self, "Helvetica")
        self.name_font.configure(size=-int(self.width / 8), weight="bold")
        self.test_font = Font(self, "Helvetica")
        self.test_font.configure(size=-int(self.width / 16), weight="bold")

        self.canvas = TargetCanvas(self, self.width)

        self.canvas.grid(row=0, column=0)
        self.info_canvas = tk.Canvas(
            self, bg="white", height=self.width / 2, width=self.width
        )
        self.info_canvas.create_text(
            self.width / 2,
            self.width / 16,
            fill="black",
            font=self.test_font,
            text=self.container.container.parent.users[match.shooter].name,
            tag="name",
        )
        self.info_canvas.create_text(
            self.width / 2,
            3 * self.width / 16,
            fill="black",
            font=self.name_font,
            text=str(
                round(self.match.summe, 1)
                if self.container.competition.decimal
                else self.match.summe_ganz
            ),
            tag="result",
        )
        self.info_canvas.create_text(
            self.width / 2,
            4.5 * self.width / 16,
            fill="black",
            font=(f"Helvetica {-int(self.width/16)}"),
            text=f"{self.match.anzahl} / {self.container.competition.count}",
            tag="count",
        )
        for n, series in enumerate(self.match.series):
            self.info_canvas.create_text(
                (n % 4 + 1) * self.width / 5,
                3 * self.width / 8 + ((self.width / 16) if n > 3 else 0),
                fill="black",
                font=(f"Helvetica {-int(self.width/16)}"),
                text=str(
                    round(series.summe, 1)
                    if self.container.competition.decimal
                    else series.summe_ganz
                ),
                tag="series",
            )

        self.info_canvas.grid(column=0, row=1)
        self.update_frame()

    def update_frame(self):
        self.canvas.draw_target(self.match.type_of_target)
        self.canvas.redraw_shots(self.match)
