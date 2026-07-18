from dataclasses import dataclass


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_kw(box: Box) -> int:
    match box:
        case Box(a=xa, b=xb):
            return xa.run() + xb.run()
    return 0


def use_kw_a(box: Box) -> int:
    match box:
        case Box(a=xa):
            return xa.run()
    return 0


def use_kw_as(box: Box) -> int:
    match box:
        case Box(a=xa as x):
            return x.run()
    return 0


def use_plain_class(box: Box) -> int:
    # Same fieldIndex path without relying on subject typing.
    match box:
        case Box(b=xb, a=xa):
            return xa.run() + xb.run()
    return 0
