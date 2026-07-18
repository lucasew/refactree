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


def use_list(box: Box) -> int:
    return list(astuple(box))[0].execute() + list(astuple(box))[1].run()


def use_tuple(box: Box) -> int:
    return tuple(astuple(box))[0].execute() + tuple(astuple(box))[1].run()


def use_dc_list(box: Box) -> int:
    return list(dataclasses.astuple(box))[0].execute() + list(dataclasses.astuple(box))[1].run()


def use_list_replace(box: Box) -> int:
    return list(astuple(replace(box)))[0].execute() + list(astuple(replace(box)))[1].run()


def use_var(box: Box) -> int:
    xs = list(astuple(box))
    return xs[0].execute() + xs[1].run()


def use_tuple_var(box: Box) -> int:
    xs = tuple(dataclasses.astuple(box))
    return xs[0].execute() + xs[1].run()


def use_field_var(box: Box) -> int:
    xa = list(astuple(box))[0]
    xb = tuple(astuple(box))[1]
    return xa.execute() + xb.run()


def use_unpack(box: Box) -> int:
    ya, yb = list(astuple(box))
    return ya.execute() + yb.run()


def use_dc_unpack(box: Box) -> int:
    ya, yb = tuple(dataclasses.astuple(box))
    return ya.execute() + yb.run()
