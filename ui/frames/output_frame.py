import datetime
import os
import math
import sys
from tkinter import ttk
from fpdf import FPDF
import numpy as np
import cv2
import tempfile
from data.series import Series
from data.competition import Competition
from data.match import RADIUS_DICT, Match
import seaborn as sns
import pandas as pd
import qrcode
from idlelib.tooltip import Hovertip

from ui.frames.default_frame import DefaultFrame


class OutputFrame(DefaultFrame):
    def __init__(self, container, parent):
        super().__init__(container, parent)

        # field options
        options = {"padx": 5, "pady": 0}
        self.columnconfigure(0, weight=1)

        self.pdf_button = ttk.Button(
            self,
            text="PDF erzeugen",
            command=self.export_pdf,
        )
        self.pdf_button.grid(row=0, column=1, sticky="e")
        Hovertip(
            self.pdf_button, "Erzeugt einen Ausdruck des Ergebinisses im PDF-Format"
        )

        self.csv_button = ttk.Button(
            self,
            text="CSV erzeugen",
            command=self.export_csv,
        )
        self.csv_button.grid(row=1, column=1, sticky="e")
        Hovertip(
            self.csv_button,
            "Speichert die Daten des Ergebnisses im CSV-Format",
        )

        self.graph_button = ttk.Button(
            self,
            text="Graph erzeugen",
            command=self.export_graphs,
        )
        self.graph_button.grid(row=2, column=1, sticky="e")
        Hovertip(
            self.graph_button,
            "Stellt die Schussverteilung graphisch dar",
        )
        self.qr_button = ttk.Button(
            self,
            text="QR-Code erzeugen",
            command=self.export_qr,
        )
        self.qr_button.grid(row=3, column=1, sticky="e")
        Hovertip(self.qr_button, "Stellt das Ergebnis als QR-Code dar")

        self.grid(column=1, row=0, padx=5, pady=5, sticky="nsew")

    def reset(self):
        pass

    def export_pdf(self):
        self.actionPDF()

    def export_csv(self):
        self.actionCSV()

    def actionCSV(self):
        self._makeCSV()

    def actionPDF(self):
        self._makeShooterPDF()

    def export_graphs(self):
        self._draw_histogram()

    def export_qr(self):
        self._draw_qr()

    def _makeCSV(self):
        filedate = datetime.datetime.strptime(self.current_match.date, "%d.%m.%Y")
        filepath = (
            self.users[self.current_match.shooter].name.replace(" ", "_") + "\\csv"
        )
        filename = datetime.datetime.strftime(filedate, "%y%m%d")
        outdir = os.getcwd()
        newpath = os.path.join(outdir, "output", filepath)
        if not os.path.exists(newpath):
            os.makedirs(newpath)
        testfilename = f"{filename}.csv"
        n = 1
        while os.path.exists(os.path.join(newpath, testfilename)):
            testfilename = f"{filename}_{n}.csv"
            n += 1

        csvfile = open(os.path.join(newpath, testfilename), "w")
        csvfile.write(
            f"{self.current_match.date};{self.current_match.anzahl};{self.current_match.type_of_target}\r"
        )
        for s in self.current_match:
            csvfile.write(
                f"{str(s.ringe)};{str(s.teiler)};{str(s.x)};{str(s.y)}\r".replace(
                    ".", ","
                )
            )
        csvfile.close()

    def _makeShooterPDF(self):
        filedate = datetime.datetime.strptime(self.current_match.date, "%d.%m.%Y")
        filepath = self.users[self.current_match.shooter].name.replace(" ", "_")
        filename = f"""{datetime.datetime.strftime(filedate, "%y%m%d")}_{self.current_match.type_of_target}"""
        outdir = os.getcwd()
        pdf = FPDF()
        pdf.add_page()
        pdf.set_font("Helvetica", "B", 16)
        pdf.text(10, 20, f"{self.competition.name} {self.current_match.date}")
        pdf.set_font("Helvetica", "", 10)

        # create a temporary directory
        with tempfile.TemporaryDirectory() as directory:
            self.__write_totals(pdf, directory)
            self.__write_series(pdf, directory)

        newpath, testfilename = PDFgen.save_to_disk(
            self.current_match, filepath, filename, outdir, pdf
        )
        if not sys.platform == "linux":
            os.startfile(os.path.join(newpath, testfilename))

    def _draw_histogram(self):
        dataframe = pd.DataFrame(
            list(
                zip(
                    self.current_match.get_r_list(),
                    self.current_match.get_t_list(),
                    self.current_match.get_x_list(),
                    self.current_match.get_y_list(),
                )
            ),
            columns=["ringe", "teiler", "x", "y"],
        )
        grid_limit = (
            max(dataframe["x"].abs().max(), dataframe["y"].abs().max())
            + RADIUS_DICT[self.current_match.type_of_target][4] / 100
        )
        p = sns.JointGrid(
            dataframe,
            x="x",
            y="y",
            xlim=(-grid_limit, grid_limit),
            ylim=(-grid_limit, grid_limit),
        )
        p.plot_joint(
            sns.kdeplot, fill=True, bw_adjust=0.5, common_norm=True, cmap="gnuplot2"
        )
        p.plot_marginals(
            sns.boxplot,
            showmeans=True,
            meanline=True,
            medianprops={"linewidth": 2, "color": "purple"},
            meanprops={"linewidth": 2, "color": "red"},
        )
        filepath = "./output/boxplot_y.png"
        p.savefig(filepath)
        if not sys.platform == "linux":
            os.startfile(os.path.abspath(filepath))

    def __write_totals(self, pdf, directory):
        schussbild = PDFgen.drawSeries(
            self.current_match, self.current_match.type_of_target
        )
        schussbild = cv2.resize(schussbild, (800, 800), interpolation=cv2.INTER_AREA)
        cv2.imwrite(directory + "\\gesamt.png", schussbild)
        pdf.image(directory + "\\gesamt.png", 210 - 10 - 70, 10, 70)
        pdf.set_font("Helvetica", "", 10)
        pdf.text(10, 20 + 10, "Name:")
        pdf.text(10, 30 + 5, "Verein:")
        pdf.text(10, 30 + 10, "Mannschaft:")
        pdf.text(10, 50, "Ergebnis:")
        pdf.text(10, 60, "Schnitt:")
        pdf.text(10, 70, "Bester Teiler:")
        # pdf.text(70, 20 + 10, "Datum:")
        # pdf.text(70, 30 + 10, "Bewerb:")
        pdf.text(50, 50, "Ablage R/L:")
        pdf.text(50, 55, "Ablage H/T:")

        pdf.text(50, 65, "10er")
        pdf.text(60, 65, "9er")
        pdf.text(70, 65, "8er")
        pdf.text(80, 65, "7er")
        pdf.text(90, 65, "6er")
        pdf.text(100, 65, "5er")

        pdf.text(50, 70, str(self.current_match.countRing(10)))
        pdf.text(60, 70, str(self.current_match.countRing(9)))
        pdf.text(70, 70, str(self.current_match.countRing(8)))
        pdf.text(80, 70, str(self.current_match.countRing(7)))
        pdf.text(90, 70, str(self.current_match.countRing(6)))
        pdf.text(100, 70, str(self.current_match.countRing(5)))

        pdf.text(35, 20 + 10, self.users[self.current_match.shooter].name)
        pdf.text(
            35,
            30 + 5,
            self.clubs[self.current_match.club].name if self.current_match.club else "",
        )
        pdf.text(
            35,
            30 + 10,
            self.teams[self.current_match.team].name if self.current_match.team else "",
        )
        # pdf.text(85, 20 + 10, match.date)
        # pdf.text(85, 30 + 10, match.competition)competition: Competition

        if self.competition.decimal:
            pdf.set_font("Helvetica", "B", 10)
            pdf.text(35, 50, f"{self.current_match.summe:.1f}")
            pdf.text(
                35,
                60,
                f"{self.current_match.summe / self.current_match.anzahl:.2f}",
            )
            pdf.set_font("Helvetica", "", 10)
            pdf.text(35, 55, f"{self.current_match.summe_ganz:d}")
            pdf.text(
                35,
                65,
                f"{self.current_match.summe_ganz / self.current_match.anzahl:.2f}",
            )
        else:
            pdf.set_font("Helvetica", "B", 10)
            pdf.text(35, 50, f"{self.current_match.summe_ganz:d}")
            pdf.text(
                35,
                60,
                f"{self.current_match.summe_ganz / self.current_match.anzahl:.2f}",
            )
            pdf.set_font("Helvetica", "", 10)
            pdf.text(35, 55, f"{self.current_match.summe:.1f}")
            pdf.text(
                35,
                65,
                f"{self.current_match.summe / self.current_match.anzahl:.2f}",
            )

        pdf.set_font("Helvetica", "B", 10)
        pdf.text(35, 70, f"{self.current_match.best.teiler:.1f}")
        pdf.set_font("Helvetica", "", 10)
        pdf.text(75, 50, f"{self.current_match.ablageRL:.1f}")
        pdf.text(75, 55, f"{self.current_match.ablageHT:.1f}")

    def __write_series(self, pdf, directory):
        abstSerie = 35
        start_y = 80
        for i, series in enumerate(self.current_match.series[:6]):
            self.__write_single_series(pdf, directory, abstSerie, start_y, i, series, 0)
        if len(self.current_match.series) > 6:
            following_pages = (len(self.current_match.series) - 6) // 8 + 1
            for j in range(0, following_pages):
                pdf.add_page()
                for i, series in enumerate(
                    self.current_match.series[6 + j : 6 + j + 8]
                ):
                    series_offset = j * 8 + 6
                    if i + series_offset > len(self.current_match.series) - 1:
                        break
                    self.__write_single_series(
                        pdf,
                        directory,
                        abstSerie,
                        10,
                        i,
                        series,
                        series_offset,
                    )

    def __write_single_series(
        self,
        pdf,
        directory,
        abstSerie,
        start_y,
        i,
        series: Series,
        series_offset,
    ):
        seriesname = directory + "\\serie" + str(i + series_offset) + ".png"
        schussbild = PDFgen.drawSeries(series, self.current_match.type_of_target)
        schussbild = cv2.resize(schussbild, (400, 400), interpolation=cv2.INTER_AREA)
        cv2.imwrite(seriesname, schussbild)

        pdf.image(seriesname, 210 - 10 - abstSerie, abstSerie * i + start_y, abstSerie)
        for j, shot in enumerate(series.shots):
            shotname = (
                directory
                + "\\serie"
                + str(i + series_offset)
                + "shot"
                + str(j)
                + ".png"
            )
            horizontal_distance = 11.5
            arrow_size = 4
            arrow_offset_y = 2
            arrow = PDFgen.drawArrow(shot.x, shot.y, self.current_match.type_of_target)
            cv2.imwrite(shotname, arrow)
            if self.competition.decimal:
                pdf.set_font("Helvetica", "B", 10)
                pdf.text(
                    horizontal_distance * j + 50,
                    abstSerie * i + start_y + 5,
                    str(shot.ringe),
                )

                pdf.set_font("Helvetica", "", 10)
                pdf.text(
                    horizontal_distance * j + 50,
                    abstSerie * i + start_y + 10,
                    str(math.floor(shot.ringe)),
                )

            else:
                pdf.set_font("Helvetica", "B", 10)
                pdf.text(
                    horizontal_distance * j + 50,
                    abstSerie * i + start_y + 5,
                    str(math.floor(shot.ringe)),
                )
                pdf.set_font("Helvetica", "", 10)
                pdf.text(
                    horizontal_distance * j + 50,
                    abstSerie * i + start_y + 10,
                    str(shot.ringe),
                )
            pdf.image(
                shotname,
                horizontal_distance * j + 50 + (4 if shot.ringe >= 10 else 2),
                abstSerie * i
                + start_y
                + arrow_offset_y
                + (5 if self.competition.decimal else 0),
                arrow_size,
                arrow_size,
            )

            seriesname = f"Serie {i+series_offset+1}"

            pdf.text(10, abstSerie * i + start_y + 5, seriesname)
            pdf.text(10, abstSerie * i + start_y + 5 + 10, "Ergebnis:")
            pdf.text(10, abstSerie * i + start_y + 5 + 20, "Bester Teiler:")
            pdf.text(50, abstSerie * i + start_y + 5 + 15, "Ablage R/L:")
            pdf.text(50, abstSerie * i + start_y + 5 + 20, "Ablage H/T:")
            pdf.line(10, abstSerie * i + start_y, pdf.w - 10, abstSerie * i + start_y)
            if i - series_offset == 3 and self.current_match.anzahl % 40 > 1:
                pdf.text(90, abstSerie * i + start_y + 5 + 15, "Zwischenergebnis:")
                pdf.text(90, abstSerie * i + start_y + 5 + 20, "Schnitt:")
                sum40 = sum(
                    [
                        (A.summe if self.competition.decimal else A.summe_ganz)
                        for A in self.current_match.series[
                            series_offset : series_offset + 4
                        ]
                    ]
                )
                pdf.text(
                    130,
                    abstSerie * i + start_y + 5 + 15,
                    (f"{sum40:.1f}" if self.competition.decimal else f"{sum40:d}"),
                )
                pdf.text(
                    130,
                    abstSerie * i + start_y + 5 + 20,
                    f"{sum40 / 40:.2f}",
                )
            if self.competition.decimal:
                pdf.set_font("Helvetica", "B", 10)
                pdf.text(
                    35,
                    abstSerie * i + start_y + 5 + 10,
                    f"{self.current_match.series[i + series_offset].summe:.1f}",
                )
                pdf.set_font("Helvetica", "", 10)
                pdf.text(
                    35,
                    abstSerie * i + start_y + 5 + 15,
                    f"{self.current_match.series[i + series_offset].summe_ganz:d}",
                )
            else:
                pdf.set_font("Helvetica", "B", 10)
                pdf.text(
                    35,
                    abstSerie * i + start_y + 5 + 10,
                    f"{self.current_match.series[i + series_offset].summe_ganz:d}",
                )
                pdf.set_font("Helvetica", "", 10)
                pdf.text(
                    35,
                    abstSerie * i + start_y + 5 + 15,
                    f"{self.current_match.series[i + series_offset].summe:.1f}",
                )
            pdf.text(
                35,
                abstSerie * i + start_y + 5 + 20,
                f"{self.current_match.series[i + series_offset].best.teiler:.1f}",
            )
            pdf.text(
                70,
                abstSerie * i + start_y + 5 + 15,
                f"{self.current_match.series[i + series_offset].ablageRL:.1f}",
            )
            pdf.text(
                70,
                abstSerie * i + start_y + 5 + 20,
                f"{self.current_match.series[i + series_offset].ablageHT:.1f}",
            )

    def _draw_qr(self):
        qrstr = PDFgen.get_compressed_string(self.current_match)
        if len(qrstr) < 4296:
            qr = qrcode.make(qrstr)
        filepath = "./output/qr.png"
        qr.save(filepath)
        if not sys.platform == "linux":
            os.startfile(os.path.abspath(filepath))


class PDFgen:
    # unused
    @staticmethod
    def drawHeatmap(series, scheibentyp):
        radiusTen = RADIUS_DICT[scheibentyp][0]
        incrementRing = RADIUS_DICT[scheibentyp][2]
        radiusCalibre = RADIUS_DICT[scheibentyp][4]

        w = 2 * (radiusTen + 9 * incrementRing)
        scale = 400 / w
        center_coordinates = (int(w / 2), int(w / 2))
        image = np.zeros((400, 400, 1), np.float64)
        max_x = max([abs(s.x) for s in series])
        max_y = max([abs(s.y) for s in series])
        radius_worst = (max_x**2 + max_y**2) ** 0.5 + radiusCalibre
        rangemin = max(int((w / 2 - radius_worst) * scale + 0.5), 0)
        rangemax = min(int((w / 2 + radius_worst) * scale + 0.5), int(w * scale + 0.5))
        for i in range(rangemin, rangemax + 1):
            for j in range(rangemin, rangemax + 1):
                for e in series:
                    distance = math.sqrt(
                        (i - (e.y + center_coordinates[1]) * scale) ** 2
                        + (j - (e.x + center_coordinates[0]) * scale) ** 2
                    )

                    image[399 - i, j] += math.exp(
                        -((distance / (0.75 * radiusCalibre * scale)) ** 2)
                    )
        normimage = cv2.normalize(
            image[rangemin:rangemax, rangemin:rangemax],
            None,
            0,
            255,
            cv2.NORM_MINMAX,
            cv2.CV_8UC1,
        )
        color_image = cv2.applyColorMap(normimage, cv2.COLORMAP_JET)
        color_image = cv2.resize(color_image, (800, 800), interpolation=cv2.INTER_AREA)
        return color_image

    @staticmethod
    def drawSeries(series: Match | Series, scheibentyp):
        radiusTen = RADIUS_DICT[scheibentyp][0]
        incrementRing = RADIUS_DICT[scheibentyp][2]
        radiusCalibre = RADIUS_DICT[scheibentyp][4]

        w = 2 * (radiusTen + 9 * incrementRing)

        green = (0, 255, 0)
        black = (0, 0, 0)
        red = (0, 0, 255)
        blue = (255, 0, 0)
        orange = (0, 136, 255)
        filled = -1
        image = PDFgen.drawTarget(scheibentyp)
        for shot in series:
            coord = (int(w / 2 + shot.x * 100), int(w / 2 - shot.y * 100))
            cv2.circle(
                image,
                coord,
                radiusCalibre,
                # orange if shot in series.ausreisser else green,
                green,
                filled,
            )
            cv2.circle(image, coord, radiusCalibre, black, 8)

        coord = (int(w / 2 + series.best.x * 100), int(w / 2 - series.best.y * 100))
        cv2.circle(image, coord, radiusCalibre, red, filled)
        cv2.circle(image, coord, radiusCalibre, black, 10)
        coord = (int(w / 2 + series.ablageRL * 100), int(w / 2 - series.ablageHT * 100))
        cv2.circle(image, coord, int(radiusCalibre / 3), blue, filled)
        cv2.circle(image, coord, int(radiusCalibre / 3), black, 10)

        max_x = max([abs(s.x * 100) for s in series])
        max_y = max([abs(s.y * 100) for s in series])
        radius_worst = int((max_x**2 + max_y**2) ** 0.5 + 0.5) + radiusCalibre
        rangemin = max(int(image.shape[0] / 2) - radius_worst, 0)
        rangemax = min(int(image.shape[0] / 2) + radius_worst, int(image.shape[0]))
        return image[rangemin:rangemax, rangemin:rangemax]

    @staticmethod
    def drawTarget(scheibentyp):
        radiusTen = RADIUS_DICT[scheibentyp][0]
        radiusInnerTen = max(RADIUS_DICT[scheibentyp][1], 0)
        incrementRing = RADIUS_DICT[scheibentyp][2]
        radiusBlack = RADIUS_DICT[scheibentyp][3]
        w = 2 * (radiusTen + 9 * incrementRing)
        blank_image = np.zeros((w, w, 3), np.uint8)
        blank_image.fill(255)
        # blank_image[:] = (255, 0, 255)
        center_coordinates = (int(w / 2), int(w / 2))
        # Black color in BGR
        black = (0, 0, 0)
        white = (255, 255, 255)
        # Line thickness of -1 px
        filled = -1

        # Using cv2.circle() method
        # Draw a circle of red color of thickness -1 px
        blank_image = cv2.circle(
            blank_image, center_coordinates, radiusBlack, black, filled
        )
        cv2.circle(blank_image, center_coordinates, radiusInnerTen, white, 10)
        for i in range(0, 10):
            cv2.circle(
                blank_image,
                center_coordinates,
                radiusTen + i * incrementRing,
                white if radiusTen + i * incrementRing < radiusBlack else black,
                10,
            )
        number_thickness = int(incrementRing * 0.04)
        number_height = cv2.getFontScaleFromHeight(
            cv2.FONT_HERSHEY_SIMPLEX, int(incrementRing * 0.4), number_thickness
        )
        for i in range(8, 0, -1):
            number_width = cv2.getTextSize(
                str(i), cv2.FONT_HERSHEY_SIMPLEX, number_height, number_thickness
            )
            number_center_radius = int(
                (9 - i) * incrementRing + radiusTen + 0.5 * incrementRing
            )
            number_offset_x = -number_width[0][0] / 2
            number_offset_y = number_width[0][1] / 2
            number_coordinates = (
                int(center_coordinates[0] + number_offset_x + 0.5),
                int(
                    center_coordinates[1] + number_offset_y + number_center_radius + 0.5
                ),
            )

            cv2.putText(
                blank_image,
                str(i),
                number_coordinates,
                cv2.FONT_HERSHEY_SIMPLEX,
                number_height,
                white if number_center_radius < radiusBlack else black,
                number_thickness,
                cv2.LINE_AA,
            )
            number_coordinates = (
                int(center_coordinates[0] + number_offset_x + 0.5),
                int(
                    center_coordinates[1] + number_offset_y - number_center_radius + 0.5
                ),
            )

            cv2.putText(
                blank_image,
                str(i),
                number_coordinates,
                cv2.FONT_HERSHEY_SIMPLEX,
                number_height,
                white if number_center_radius < radiusBlack else black,
                number_thickness,
                cv2.LINE_AA,
            )
            number_coordinates = (
                int(
                    center_coordinates[0] + number_offset_x + number_center_radius + 0.5
                ),
                int(center_coordinates[1] + number_offset_y + 0.5),
            )

            cv2.putText(
                blank_image,
                str(i),
                number_coordinates,
                cv2.FONT_HERSHEY_SIMPLEX,
                number_height,
                white if number_center_radius < radiusBlack else black,
                number_thickness,
                cv2.LINE_AA,
            )
            number_coordinates = (
                int(
                    center_coordinates[0] + number_offset_x - number_center_radius + 0.5
                ),
                int(center_coordinates[1] + number_offset_y + 0.5),
            )

            cv2.putText(
                blank_image,
                str(i),
                number_coordinates,
                cv2.FONT_HERSHEY_SIMPLEX,
                number_height,
                white if number_center_radius < radiusBlack else black,
                number_thickness,
                cv2.LINE_AA,
            )

        return blank_image

    @staticmethod
    def drawArrow(x, y, scheibentyp):
        radiusInnerTen = RADIUS_DICT[scheibentyp][1]
        radiusCalibre = RADIUS_DICT[scheibentyp][4]
        w = 50  # 50
        t = 2
        r = int(w / 2 - 2 * t)
        is_inner_ten = (
            (x**2 + y**2) ** 0.5 - radiusCalibre / 100
        ) < radiusInnerTen / 100
        alpha = math.atan2(y, x)
        center_coordinates = (int(w / 2), int(w / 2))
        blank_image = np.zeros((w, w, 1), np.uint8)
        blank_image.fill(255)
        if is_inner_ten:
            blank_image = cv2.circle(blank_image, center_coordinates, r, 0, t)
        if not (x == 0 and y == 0):
            x_factor = math.cos(alpha)
            y_factor = math.sin(alpha)
        else:
            x_factor = 0
            y_factor = 0
        start_point = (int(w / 2 - r * x_factor + 0.5), int(w / 2 + r * y_factor + 0.5))
        end_point = (int(w / 2 + r * x_factor + 0.5), int(w / 2 - r * y_factor + 0.5))
        blank_image = cv2.arrowedLine(
            blank_image, start_point, end_point, color=0, thickness=t, tipLength=0.3
        )

        return blank_image

    @staticmethod
    def save_to_disk(match: Match, filepath, filename, outdir, pdf):
        newpath = os.path.join(outdir, "output", filepath, match.type_of_target)
        if not os.path.exists(newpath):
            os.makedirs(newpath)
        testfilename = f"{filename}.pdf"
        n = 1
        while os.path.exists(os.path.join(newpath, testfilename)):
            testfilename = f"{filename}_{n}.pdf"
            n += 1
        pdf.output(os.path.join(newpath, testfilename), "F")
        return newpath, testfilename

    @staticmethod
    def get_compressed_string(match: Match):
        def map_ring(instr: str) -> str:
            return (
                instr.replace("10.", "A")
                .replace("9.", "B")
                .replace("8.", "C")
                .replace("7.", "D")
                .replace("6.", "E")
                .replace("5.", "F")
                .replace("4.", "G")
                .replace("3.", "H")
                .replace("2.", "I")
                .replace("1.", "J")
            )

        outstr = f"""{match.date.replace(".","")}{match.type_of_target}"""
        for s in match:
            line = f"{map_ring(str(s.ringe))}{str(s.teiler)}+{str(s.x)}+{str(s.y)}".replace(
                "+-", "-"
            )
            outstr += line
        return outstr
