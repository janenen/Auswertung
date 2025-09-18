from tkinter import ttk
from data.series import Series
from idlelib.tooltip import Hovertip
from ui.utils.target import TargetCanvas
from .default_frame import DefaultFrame


class UserResultFrame(DefaultFrame):
    actuallist: Series

    def __init__(self, container, parent):
        super().__init__(container, parent)
        # field options
        options = {"padx": 5, "pady": 0}
        self.i = -1
        self.shotcircles = {}
        self.canvsize = 220
        self.canvas = TargetCanvas(self, self.canvsize)
        self.canvas.place(height=self.canvsize, width=self.canvsize)
        self.export = False

        self.backbutton = ttk.Button(self, text="<", command=self.actionBack)
        self.backbutton.place(
            x=0, y=self.canvsize, height=self.canvsize / 8, width=self.canvsize / 4
        )
        Hovertip(self.backbutton, "Zu vorheriger Serie / Gesammtdarstellung wechseln")
        self.serieslabel = ttk.Label(self, text="Gesammt", anchor="center")
        self.serieslabel.place(
            x=self.canvsize / 4,
            y=self.canvsize,
            height=self.canvsize / 8,
            width=self.canvsize * 2 / 4,
        )
        self.forebutton = ttk.Button(self, text=">", command=self.actionFore)
        self.forebutton.place(
            x=self.canvsize * 3 / 4,
            y=self.canvsize,
            height=self.canvsize / 8,
            width=self.canvsize / 4,
        )
        Hovertip(self.forebutton, "Zu nächster Serie / Gesammtdarstellung wechseln")

        self.resultlabelframe = ttk.LabelFrame(self, text="Gesamtergebnis")
        ttk.Label(self.resultlabelframe, text="Ergebnis: ").grid(
            row=0, column=0, sticky="e"
        )
        self.resultlabel = ttk.Label(self.resultlabelframe, text="-")
        self.resultlabel.grid(row=0, column=1, sticky="w")
        ttk.Label(self.resultlabelframe, text="Schnitt: ").grid(
            row=1, column=0, sticky="e"
        )
        self.schnittlabel = ttk.Label(self.resultlabelframe, text="-")
        self.schnittlabel.grid(row=1, column=1, sticky="w")
        ttk.Label(self.resultlabelframe, text="Ablage R/L: ").grid(
            row=2, column=0, sticky="e"
        )
        self.devxlabel = ttk.Label(self.resultlabelframe, text="-")
        self.devxlabel.grid(row=2, column=1, sticky="w")
        ttk.Label(self.resultlabelframe, text="Ablage H/T: ").grid(
            row=3, column=0, sticky="e"
        )
        self.devylabel = ttk.Label(self.resultlabelframe, text="-")
        self.devylabel.grid(row=3, column=1, sticky="w")
        self.resultlabelframe.place(
            x=self.canvsize,
            y=self.canvsize * 0 / 9,
            height=self.canvsize * 5 / 10,
            width=self.canvsize * 3 / 4,
        )

        self.infolabelframe = ttk.LabelFrame(self, text="Schusswert")
        ttk.Label(self.infolabelframe, text="Ringe: ").grid(
            column=0, row=0, sticky="e", **options
        )
        self.ringlabel = ttk.Label(self.infolabelframe, text="-")
        self.ringlabel.grid(column=1, row=0, sticky="w")
        ttk.Label(self.infolabelframe, text="X: ").grid(
            column=0, row=1, sticky="e", **options
        )
        self.xlabel = ttk.Label(self.infolabelframe, text="-")
        self.xlabel.grid(column=1, row=1, sticky="w")
        ttk.Label(self.infolabelframe, text="Y: ").grid(
            column=0, row=2, sticky="e", **options
        )
        self.ylabel = ttk.Label(self.infolabelframe, text="-")
        self.ylabel.grid(column=1, row=2, sticky="w")
        ttk.Label(self.infolabelframe, text="Teiler: ").grid(
            column=0, row=3, sticky="e", **options
        )
        self.teilerlabel = ttk.Label(self.infolabelframe, text="-")
        self.teilerlabel.grid(column=1, row=3, sticky="w")
        self.infolabelframe.place(
            x=self.canvsize,
            y=self.canvsize * 4 / 9,
            height=self.canvsize * 5 / 10,
            width=self.canvsize * 3 / 4,
        )
        self.shotlabellist = []

        self.inputlabelframe = ttk.LabelFrame(self, text="Speichern")
        # user input
        self.generate_button = ttk.Button(
            self.inputlabelframe,
            text="Exportieren",
            command=self.action_export,
        )
        self.generate_button.grid(column=0, row=0, sticky="we")
        self.inputlabelframe.place(
            x=self.canvsize,
            y=self.canvsize * 8 / 9,
            height=self.canvsize * 3 / 9,
            width=self.canvsize * 3 / 4,
        )
        Hovertip(
            self.generate_button,
            "Ergebnis speichern\nDas Ergebnis wird gespeichert und ein druckbarer Bericht geöffnet",
        )
        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def redraw(self):
        if self.i < 0:
            self.actuallist = Series(self.current_match.shots)
            self.serieslabel.config(text="Gesammt")
            self.resultlabelframe.config(text="Gesammtergebnis")
        else:
            self.actuallist = self.current_match.series[self.i]
            self.serieslabel.config(text=f"Serie {self.i+1}")
            self.resultlabelframe.config(text=f"Serie {self.i+1}")
        self.canvas.draw_target(self.current_match.type_of_target)
        self.canvas.redraw_shots(self.current_match, self.i)
        self.write_shots()

    def actionFore(self):
        self.i = ((self.i + 2) % (len(self.current_match.series) + 1)) - 1
        self.redraw()

    def actionBack(self):
        self.i = ((self.i) % (len(self.current_match.series) + 1)) - 1
        self.redraw()

    def action_export(self):
        self.export = True
        self.proceed()

    def update_text(self, event):
        id = self.canvas.find_withtag("current")[0]
        self.ringlabel.config(
            text=(
                f"{self.shotcircles[id].ringe:.1f}"
                if self.competition.decimal
                else f"{self.shotcircles[id].ringe_ganz}"
            )
        )
        self.xlabel.config(text=f"{self.shotcircles[id].x:.2f}")

        self.ylabel.config(text=f"{self.shotcircles[id].y:.2f}")

        self.teilerlabel.config(text=f"{self.shotcircles[id].teiler:.1f}")

    def write_shots(self):
        self.resultlabel.config(
            text=(
                f"{self.actuallist.summe:.1f}"
                if self.competition.decimal
                else f"{self.actuallist.summe_ganz:d}"
            )
        )
        self.schnittlabel.config(
            text=(
                f"{self.actuallist.summe / self.actuallist.anzahl:.2f}"
                if self.competition.decimal
                else f"{self.actuallist.summe_ganz / self.actuallist.anzahl:.2f}"
            )
        )
        self.devxlabel.config(text=f"{self.actuallist.ablageRL:.1f}")
        self.devylabel.config(text=f"{self.actuallist.ablageHT:.1f}")
        for label in self.shotlabellist:
            label.destroy()
        self.shotlabellist = []
        for n, shot in enumerate(self.current_match.shots):
            label = ttk.Label(
                self,
                text=(
                    f"{shot.ringe:.1f}"
                    if self.competition.decimal
                    else f"{shot.ringe_ganz:d}"
                ),
                anchor="center",
            )
            label.place(
                x=(15 / 8 + int(n / 10) / 6) * self.canvsize,
                y=((n % 10) / 11) * self.canvsize,
                height=self.canvsize / 11,
                width=self.canvsize / 6,
            )
            self.shotlabellist.append(label)
            if n > 0 and (n + 1) % 10 == 0:
                end_index = n + 1
                start_index = end_index - 10
                current_series = Series(self.current_match.shots[start_index:end_index])
                label = ttk.Label(
                    self,
                    text=(
                        f"{current_series.summe:.1f}"
                        if self.competition.decimal
                        else f"{current_series.summe_ganz:d}"
                    ),
                    anchor="center",
                )
                label.place(
                    x=(15 / 8 + (n // 10) / 6) * self.canvsize,
                    y=(11 / 11) * self.canvsize,
                    height=self.canvsize / 11,
                    width=self.canvsize / 6,
                )
                self.shotlabellist.append(label)

    def reset(self):
        self.activate_ok_button()
        self.deactivate_back_button()
        if not self.current_match:
            if self.competition:
                self.current_match = self.matches[self.competition.entries[-1]]

        self.actuallist = Series(self.current_match.shots)
        self.parent.competitions_frame.competition_listbox.configure(state="normal")
        self.parent.competitions_frame.update_entries()
        self.serieslabel.config(text="Gesammt")
        self.resultlabelframe.config(text="Gesammtergebnis")
        self.i = -1
        self.write_shots()

        # self.l_slider.set(1)
        self.canvas.draw_target(self.current_match.type_of_target)
        self.canvas.redraw_shots(self.current_match)
