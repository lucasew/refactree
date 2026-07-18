from dataclasses import dataclass, asdict
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


@dataclass
class Pair:
    # homogeneous — both slots rename
    a: A
    c: A


def use_unpack(box: Box) -> int:
    xa, xb = asdict(box).values()
    return xa.execute() + xb.run()


def use_list(box: Box) -> int:
    xa, xb = list(asdict(box).values())
    return xa.execute() + xb.run()


def use_tuple(box: Box) -> int:
    xa, xb = tuple(asdict(box).values())
    return xa.execute() + xb.run()


def use_dc(box: Box) -> int:
    xa, xb = dataclasses.asdict(box).values()
    return xa.execute() + xb.run()


def use_vars(box: Box) -> int:
    xa, xb = vars(box).values()
    return xa.execute() + xb.run()


def use_dunder(box: Box) -> int:
    xa, xb = box.__dict__.values()
    return xa.execute() + xb.run()


def use_assigned(box: Box) -> int:
    d = asdict(box)
    xa, xb = d.values()
    return xa.execute() + xb.run()


def use_assigned_list(box: Box) -> int:
    d = asdict(box)
    xa, xb = list(d.values())
    return xa.execute() + xb.run()


def use_star(box: Box) -> int:
    xa, *rest = asdict(box).values()
    return xa.execute()


def use_homo(pair: Pair) -> int:
    xa, xc = asdict(pair).values()
    return xa.execute() + xc.execute()


def use_index_still(box: Box) -> int:
    # keep B leaf untouched via key path
    return asdict(box)["b"].run()
