from dataclasses import dataclass, asdict
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


def use_asdict(box: Box) -> int:
    return asdict(box)["a"].run() + asdict(box)["b"].run()


def use_dc_asdict(box: Box) -> int:
    return dataclasses.asdict(box)["a"].run() + dataclasses.asdict(box)["b"].run()


def use_vars(box: Box) -> int:
    return vars(box)["a"].run() + vars(box)["b"].run()


def use_dunder(box: Box) -> int:
    return box.__dict__["a"].run() + box.__dict__["b"].run()


def use_field_var(box: Box) -> int:
    xa = asdict(box)["a"]
    xb = vars(box)["b"]
    return xa.run() + xb.run()


def use_dunder_field_var(box: Box) -> int:
    xa = box.__dict__["a"]
    xb = box.__dict__["b"]
    return xa.run() + xb.run()
