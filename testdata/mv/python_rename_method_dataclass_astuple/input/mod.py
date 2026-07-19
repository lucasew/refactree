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


def use_var(box: Box) -> int:
    t = astuple(box)
    return t[0].run() + t[1].run()


def use_dc_var(box: Box) -> int:
    t = dataclasses.astuple(box)
    return t[0].run() + t[1].run()


def use_field_var(box: Box) -> int:
    t = astuple(box)
    xa = t[0]
    xb = t[1]
    return xa.run() + xb.run()


def use_walrus(box: Box) -> int:
    if (t := astuple(box)):
        return t[0].run() + t[1].run()
    return 0
