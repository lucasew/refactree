from typing import TypedDict


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class Box(TypedDict):
    a: A
    b: B


def use_match_a(box: Box) -> int:
    match box:
        case {"a": xa}:
            return xa.execute()
    return 0


def use_match_b(box: Box) -> int:
    match box:
        case {"b": xb}:
            return xb.run()
    return 0


def use_match_both(box: Box) -> int:
    match box:
        case {"a": xa, "b": xb}:
            return xa.execute() + xb.run()
    return 0


def use_match_as(box: Box) -> int:
    match box:
        case {"a": xa as x}:
            return x.execute()
    return 0


def use_match_assigned() -> int:
    box: Box = {"a": A(), "b": B()}
    match box:
        case {"a": xa}:
            return xa.execute()
    return 0
