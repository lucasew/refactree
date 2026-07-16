from typing import TypedDict


class Box(TypedDict):
    assist: int
    stay: int


def make() -> Box:
    return Box(assist=1, stay=2)
