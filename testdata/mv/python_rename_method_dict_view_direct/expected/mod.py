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


def use_asdict(box: Box) -> int:
    return asdict(box)["a"].execute() + asdict(box)["b"].run()


def use_dc_asdict(box: Box) -> int:
    return dataclasses.asdict(box)["a"].execute() + dataclasses.asdict(box)["b"].run()


def use_vars(box: Box) -> int:
    return vars(box)["a"].execute() + vars(box)["b"].run()


def use_dunder(box: Box) -> int:
    return box.__dict__["a"].execute() + box.__dict__["b"].run()


def use_field_var(box: Box) -> int:
    xa = asdict(box)["a"]
    xb = vars(box)["b"]
    return xa.execute() + xb.run()


def use_dunder_field_var(box: Box) -> int:
    xa = box.__dict__["a"]
    xb = box.__dict__["b"]
    return xa.execute() + xb.run()
