# import configparser
# import os
import sys
import tkinter as tk
from tkinter import ttk
from data.club import Club, ClubDB
from data.competition import Competition, CompetitionDB
from data.match import Match, MatchDB
from data.user import User, UserDB
from data.team import Team, TeamDB
from machines.machine import Machine
from ui.frames.club_settings_frame import ClubSettingsFrame
from ui.frames.output_frame import OutputFrame
from ui.frames.select_club_frame import SelectClubFrame
from ui.frames.show_inactive_competitons_frame import ShowInactiveCompetitions
from .select_user_frame import SelectUserFrame
from .select_team_frame import SelectTeamFrame
from .team_settings_frame import TeamSettingsFrame
from .reading_frame import ReadingFrame
from .user_settings_frame import UserSettingsFrame
from .machine_selection_frame import MaschineSelectionFrame
from .user_result_frame import UserResultFrame
from .competition_settings_frame import CompetitionSettingsFrame
from .competition_result_frame import CompetitionResultFrame
from .competitions_frame import Competitions
from .competition_control_frame import CompetitionControlFrame


class ControlFrame(ttk.Frame):
    active_competitions: list[Competition] = []
    competitions: CompetitionDB
    competition: Competition = None
    users: UserDB
    user: User = None
    clubs: ClubDB
    club: Club = None
    matches: MatchDB
    current_match: Match = None
    source: Machine
    teams: TeamDB
    team: Team = None

    def __init__(self, container):
        super().__init__(container)
        self.container = container
        self.nextframe = "control"
        self.add_to_current_competition = True
        #  buttons
        self.back_button = ttk.Button(self, text="Zurück", command=self.actionBack)
        self.back_button.grid(column=1, row=0, padx=5, pady=5)
        self.ok_button = ttk.Button(self, text="OK", command=self.actionOK)
        self.ok_button.grid(column=2, row=0, padx=5, pady=5)
        self.grid(column=1, row=1, padx=5, pady=5, sticky="se")
        # self.reset()
        self.competitions_frame = Competitions(container, self)
        # read DB
        self.users = UserDB.load()
        self.clubs = ClubDB.load()
        self.teams = TeamDB.load()
        self.matches = MatchDB.load()
        self.competitions = CompetitionDB.load()
        self.active_competitions = self.competitions.get_active_competitions()

        # initialize frames
        self.frames: dict[str, ttk.Frame] = {
            "control": CompetitionControlFrame(container, self),
            "competition": CompetitionSettingsFrame(container, self),
            "machine": MaschineSelectionFrame(container, self),
            "user": SelectUserFrame(container, self),
            "user_settings": UserSettingsFrame(container, self),
            "reading": ReadingFrame(container, self),
            "match_result": UserResultFrame(container, self),
            "competition_result": CompetitionResultFrame(container, self),
            "select_team": SelectTeamFrame(container, self),
            "team_settings": TeamSettingsFrame(container, self),
            "select_club": SelectClubFrame(container, self),
            "club_settings": ClubSettingsFrame(container, self),
            "output": OutputFrame(container, self),
            "inactive_competitons": ShowInactiveCompetitions(container, self),
        }
        self.change_frame()

    def actionOK(self):
        if self.nextframe == "control":
            self.competitions_frame.competition_listbox.unbind("<<ListboxSelect>>")
            self.competitions_frame.competition_listbox.configure(state="disabled")
            if self.frames["control"].next_step == "add competition":
                self.add_to_current_competition = True
                self.nextframe = "competition"
            elif self.frames["control"].next_step == "finish competition":
                self.nextframe = "competition_result"
            elif self.frames["control"].next_step == "add entry":
                self.nextframe = "machine"
                self.add_to_current_competition = True
            elif self.frames["control"].next_step == "quick analysis":
                self.add_to_current_competition = False
                self.nextframe = "competition"
            elif self.frames["control"].next_step == "show entries":
                self.nextframe = "match_result"
            elif self.frames["control"].next_step == "quit":
                sys.exit(0)
            elif self.frames["control"].next_step == "show old competitions":
                self.nextframe = "inactive_competitons"

        elif self.nextframe == "competition":
            if self.frames["competition"].parseInput():
                self.nextframe = "machine"

        elif self.nextframe == "machine":
            self.nextframe = "user"

        elif self.nextframe == "user":
            if self.frames["user"].edit_user():
                self.nextframe = "user_settings"
            elif self.frames["user"].new_user():
                self.nextframe = "user_settings"
            elif self.frames["user"].team_to_select():
                self.nextframe = "select_team"
            elif self.frames["user"].club_to_select():
                self.nextframe = "select_club"
            else:
                self.nextframe = "reading"

        elif self.nextframe == "user_settings":
            if self.frames["user_settings"].parseInput():
                self.frames["user"].create_new_user = False
                self.nextframe = "user"

        elif self.nextframe == "select_team":
            if self.frames["select_team"].edit_team():
                self.nextframe = "team_settings"
            elif self.frames["select_team"].new_team():
                self.nextframe = "team_settings"
            else:
                self.nextframe = "user"

        elif self.nextframe == "team_settings":
            if self.frames["team_settings"].parseInput():
                self.nextframe = "user"

        elif self.nextframe == "select_club":
            if self.frames["select_club"].edit_club():
                self.nextframe = "club_settings"
            elif self.frames["select_club"].new_club():
                self.nextframe = "club_settings"
            else:
                self.nextframe = "user"

        elif self.nextframe == "club_settings":
            if self.frames["club_settings"].parseInput():
                self.nextframe = "user"

        elif self.nextframe == "reading":
            self.nextframe = "match_result"

        elif self.nextframe == "match_result":
            if self.frames["match_result"].export:
                self.frames["match_result"].export = False
                self.nextframe = "output"
            else:
                self.competitions_frame.competition_listbox.unbind("<<ListboxSelect>>")
                self.nextframe = "control"

        elif self.nextframe == "output":
            self.nextframe = "match_result"

        elif self.nextframe == "competition_result":
            self.competitions_frame.competition_listbox.configure(state="normal")
            self.nextframe = "control"
            self.frame.remove_current_competition()

        elif self.nextframe == "inactive_competitons":
            self.nextframe = "control"
        self.change_frame()

    def actionBack(self):
        if self.nextframe == "reading":
            self.frame.reader.shutdown = True
        self.competitions_frame.competition_listbox.configure(state="normal")
        self.nextframe = "control"
        self.change_frame()

    def reset(self):
        self.user = None
        self.team = None
        self.club = None

    def change_frame(self):
        self.frame = self.frames[self.nextframe]
        self.frame.tkraise()
        self.frame.reset()
