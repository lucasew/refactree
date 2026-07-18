from dataclasses import dataclass, asdict
import copy
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


def use_direct(box: Box) -> int:
    return copy.copy(asdict(box)["a"]).execute() + copy.copy(asdict(box)["b"]).run()


def use_dc(box: Box) -> int:
    return copy.copy(dataclasses.asdict(box)["a"]).execute() + copy.copy(dataclasses.asdict(box)["b"]).run()


def use_vars(box: Box) -> int:
    return copy.copy(vars(box)["a"]).execute() + copy.copy(vars(box)["b"]).run()


def use_dunder(box: Box) -> int:
    return copy.copy(box.__dict__["a"]).execute() + copy.copy(box.__dict__["b"]).run()


def use_get(box: Box) -> int:
    return copy.copy(asdict(box).get("a")).execute() + copy.copy(asdict(box).get("b")).run()


def use_deepcopy(box: Box) -> int:
    return copy.deepcopy(asdict(box)["a"]).execute() + copy.deepcopy(asdict(box)["b"]).run()


def use_assign(box: Box) -> int:
    xa = copy.copy(asdict(box)["a"])
    xb = copy.copy(asdict(box)["b"])
    return xa.execute() + xb.run()


def use_import_copy(box: Box) -> int:
    from copy import copy, deepcopy
    return copy(asdict(box)["a"]).execute() + deepcopy(asdict(box)["b"]).run()
