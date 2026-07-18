from dataclasses import dataclass, astuple
import dataclasses


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


def use_unpack(box: Box) -> int:
    xa, xb = astuple(box)
    return xa.run() + xb.run()


def use_dc_unpack(box: Box) -> int:
    xa, xb = dataclasses.astuple(box)
    return xa.run() + xb.run()


def use_star_unpack(box: Box) -> int:
    xa, *rest = astuple(box)
    return xa.run()
