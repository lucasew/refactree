from dataclasses import dataclass


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_pos(box: Box) -> int:
    match box:
        case Box(xa, xb):
            return xa.execute() + xb.run()
    return 0


def use_pos_a(box: Box) -> int:
    match box:
        case Box(xa):
            return xa.execute()
    return 0


def use_pos_as(box: Box) -> int:
    match box:
        case Box(xa as x):
            return x.execute()
    return 0


def use_pos_mixed(box: Box) -> int:
    match box:
        case Box(xa, b=xb):
            return xa.execute() + xb.run()
    return 0


def use_plain_class(box: Box) -> int:
    # Same fieldOrder path without relying on subject typing.
    match box:
        case Box(xa, xb):
            return xa.execute() + xb.run()
    return 0
