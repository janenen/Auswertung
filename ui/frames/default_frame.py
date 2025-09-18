from tkinter import ttk
from data.club import Club, ClubDB
from data.competition import Competition, CompetitionDB
from data.league import LeagueDB, RSBLeague
from data.match import Match, MatchDB
from data.team import Team, TeamDB
from data.user import User, UserDB
from machines.machine import Machine
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from ui.frames.control_frame import ControlFrame


class DefaultFrame(ttk.Frame):
    def __init__(self, container, parent: "ControlFrame"):
        super().__init__(container)
        self.parent = parent
        self.container: ttk.Frame = container

    @property
    def add_to_current_competition(self) -> bool:
        return self.parent.add_to_current_competition

    @add_to_current_competition.setter
    def add_to_current_competition(self, value: bool):
        self.parent.add_to_current_competition = value

    @property
    def competitions(self) -> CompetitionDB:
        return self.parent.competitions

    @competitions.setter
    def competitions(self, value: CompetitionDB):
        self.parent.competitions = value

    @property
    def active_competitions(self) -> list[Competition]:
        return self.parent.active_competitions

    @active_competitions.setter
    def active_competitions(self, value: list[Competition]):
        self.parent.active_competitions = value

    @property
    def competition(self) -> Competition:
        return self.parent.competition

    @competition.setter
    def competition(self, value: Competition):
        self.parent.competition = value

    @property
    def leagues(self) -> LeagueDB:
        return self.parent.leagues

    @property
    def league(self) -> RSBLeague:
        return self.parent.league

    @league.setter
    def league(self, value: RSBLeague):
        self.parent.league = value

    @property
    def users(self) -> UserDB:
        return self.parent.users

    @users.setter
    def users(self, value: UserDB):
        self.parent.users = value

    @property
    def user(self) -> User:
        return self.parent.user

    @user.setter
    def user(self, value: User):
        self.parent.user = value

    @users.setter
    def users(self, value: UserDB):
        self.parent.users = value

    @property
    def clubs(self) -> ClubDB:
        return self.parent.clubs

    @clubs.setter
    def club(self, value: ClubDB):
        self.parent.clubs = value

    @property
    def club(self) -> Club:
        return self.parent.club

    @club.setter
    def club(self, value: Club):
        self.parent.club = value

    @property
    def matches(self) -> MatchDB:
        return self.parent.matches

    @matches.setter
    def matches(self, value: MatchDB):
        self.parent.matches = value

    @property
    def current_match(self) -> Match:
        return self.parent.current_match

    @current_match.setter
    def current_match(self, value: Match):
        self.parent.current_match = value

    @property
    def source(self) -> Machine:
        return self.parent.source

    @source.setter
    def source(self, value: Machine):
        self.parent.source = value

    @property
    def teams(self) -> TeamDB:
        return self.parent.teams

    @teams.setter
    def teams(self, value: TeamDB):
        self.parent.teams = value

    @property
    def team(self) -> Team:
        return self.parent.team

    @team.setter
    def team(self, value: Team):
        self.parent.team = value

    def proceed(self):
        self.parent.actionOK()

    def recede(self):
        self.parent.actionBack()

    def activate_back_button(self):
        self.parent.back_button["state"] = "normal"

    def deactivate_back_button(self):
        self.parent.back_button["state"] = "disabled"

    def activate_ok_button(self):
        self.parent.ok_button["state"] = "normal"

    def deactivate_ok_button(self):
        self.parent.ok_button["state"] = "disabled"
