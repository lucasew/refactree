from dataclasses import dataclass, astuple, replace
import dataclasses


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


def use_chain(box: Box) -> int:
    return astuple(replace(box))[0].execute() + astuple(replace(box))[1].run()


def use_dc_chain(box: Box) -> int:
    return dataclasses.astuple(dataclasses.replace(box))[0].execute() + dataclasses.astuple(dataclasses.replace(box))[1].run()


def use_mixed(box: Box) -> int:
    return astuple(dataclasses.replace(box))[0].execute() + dataclasses.astuple(replace(box))[1].run()


def use_var(box: Box) -> int:
    t = astuple(replace(box))
    return t[0].execute() + t[1].run()


def use_dc_var(box: Box) -> int:
    t = dataclasses.astuple(dataclasses.replace(box))
    return t[0].execute() + t[1].run()


def use_field_var(box: Box) -> int:
    xa = astuple(replace(box))[0]
    xb = dataclasses.astuple(dataclasses.replace(box))[1]
    return xa.execute() + xb.run()


def use_unpack(box: Box) -> int:
    ya, yb = astuple(replace(box))
    return ya.execute() + yb.run()


def use_dc_unpack(box: Box) -> int:
    ya, yb = dataclasses.astuple(dataclasses.replace(box))
    return ya.execute() + yb.run()
