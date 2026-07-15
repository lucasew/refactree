from typing import TypedDict


class Box(TypedDict):
    helper: int
    stay: int


def make() -> Box:
    return Box(helper=1, stay=2)
