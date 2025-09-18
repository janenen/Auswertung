import tkinter as tk
from tkinter.font import Font
from data.match import RADIUS_DICT, Match
from data.shot import Shot


class TargetCanvas(tk.Canvas):
    def __init__(self, master, width):
        self.width = width
        self.number_font = Font(master, "Helvetica")
        super().__init__(
            master,
            bg="white",
            height=self.width,
            width=self.width,
        )

    def clear(self):
        self.delete("all")

    def draw_target(self, type_of_target="LG"):
        self.delete("ring")
        radiusTen = RADIUS_DICT[type_of_target][0]
        incrementRing = RADIUS_DICT[type_of_target][2]
        radiusBlack = RADIUS_DICT[type_of_target][3]
        w = 2 * (radiusTen + 9 * incrementRing)
        scalefactor = self.width / w
        self.create_oval(
            (-radiusBlack) * scalefactor + self.width / 2,
            radiusBlack * scalefactor + self.width / 2,
            radiusBlack * scalefactor + self.width / 2,
            -radiusBlack * scalefactor + self.width / 2,
            fill="black",
            tag="ring",
        )

        self.ringcircles = [
            self.create_oval(
                -(i * incrementRing + radiusTen) * scalefactor + self.width / 2,
                (i * incrementRing + radiusTen) * scalefactor + self.width / 2,
                (i * incrementRing + radiusTen) * scalefactor + self.width / 2,
                -(i * incrementRing + radiusTen) * scalefactor + self.width / 2,
                outline="white" if i * incrementRing < radiusBlack else "black",
                tag="ring",
            )
            for i in range(10)
        ]
        self.delete("number")
        for i in range(1, 9):
            radius = (i * incrementRing + radiusTen) + (incrementRing / 2)
            self.create_text(
                self.width / 2 + radius * scalefactor,
                self.width / 2,
                fill="white" if radius < radiusBlack else "black",
                font=self.number_font,
                text=str(9 - i),
                tag="number",
            )
            self.create_text(
                self.width / 2 - radius * scalefactor,
                self.width / 2,
                fill="white" if radius < radiusBlack else "black",
                font=self.number_font,
                text=str(9 - i),
                tag="number",
            )
            self.create_text(
                self.width / 2,
                self.width / 2 + radius * scalefactor,
                fill="white" if radius < radiusBlack else "black",
                font=self.number_font,
                text=str(9 - i),
                tag="number",
            )
            self.create_text(
                self.width / 2,
                self.width / 2 - radius * scalefactor,
                fill="white" if radius < radiusBlack else "black",
                font=self.number_font,
                text=str(9 - i),
                tag="number",
            )
        self.number_font.configure(size=int(-0.8 * incrementRing * scalefactor))

    def redraw_shots(self, match: Match, series=-1):
        self.delete("shot")

        radiusCalibre = RADIUS_DICT[match.type_of_target][4]
        radiusTen = RADIUS_DICT[match.type_of_target][0]
        incrementRing = RADIUS_DICT[match.type_of_target][2]
        w = 2 * (radiusTen + 9 * incrementRing)
        scalefactor = self.width / w
        for n, shot in enumerate(match.shots if series < 0 else match.series[series]):
            self.create_oval(
                (shot.x * 100 - radiusCalibre) * scalefactor + self.width / 2,
                -(shot.y * 100 - radiusCalibre) * scalefactor + self.width / 2,
                (shot.x * 100 + radiusCalibre) * scalefactor + self.width / 2,
                -(shot.y * 100 + radiusCalibre) * scalefactor + self.width / 2,
                fill=(
                    "green"
                    if not shot
                    == (
                        match.shots[-1] if series < 0 else match.series[series][-1]
                    )  # match.shots[-1]
                    else (
                        "red"
                        if shot.ringe_ganz == 10
                        else ("yellow" if shot.ringe_ganz == 9 else "blue")
                    )
                ),
                tag="shot",
            )
            self.create_text(
                shot.x * 100 * scalefactor + self.width / 2,
                -shot.y * 100 * scalefactor + self.width / 2,
                fill="black",
                font=self.number_font,
                text=str(n + 1),
                tag="number",
            )
        shotbox = self.bbox("shot")
        allbox = self.bbox("ring")
        size = (allbox[2] + 1) - (allbox[0] - 1)
        rect = [shotbox[i] - size // 2 for i in range(len(shotbox))]
        r = max(abs(min(rect)), max(rect))
        scale = (size / 2) / r
        self.scale(
            "all",
            self.width / 2,
            self.width / 2,
            scale,
            scale,
        )
        self.number_font.configure(size=int(-0.8 * incrementRing * scalefactor * scale))

    def draw_single_shot(self, shot: Shot, type_of_target):
        self.itemconfigure("shot", fill="green")
        radiusCalibre = RADIUS_DICT[type_of_target][4]
        radiusTen = RADIUS_DICT[type_of_target][0]
        incrementRing = RADIUS_DICT[type_of_target][2]
        w = 2 * (radiusTen + 9 * incrementRing)
        scalefactor = self.width / w

        self.create_oval(
            (shot.x * 100 - radiusCalibre) * scalefactor + self.width / 2,
            -(shot.y * 100 - radiusCalibre) * scalefactor + self.width / 2,
            (shot.x * 100 + radiusCalibre) * scalefactor + self.width / 2,
            -(shot.y * 100 + radiusCalibre) * scalefactor + self.width / 2,
            fill=(
                "red"
                if shot.ringe_ganz == 10
                else ("yellow" if shot.ringe_ganz == 9 else "blue")
            ),
            tag="shot",
        )
        # self.create_text(
        #    shot.x * 100* scalefactor+ self.width / 2,
        #    -shot.y * 100* scalefactor+ self.width / 2,
        #    fill= "black",
        #    font=self.number_font,
        #    text=str(n+1),
        #    tag="number",
        # )
