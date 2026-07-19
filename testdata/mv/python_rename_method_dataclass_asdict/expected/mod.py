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


def use_var(box: Box) -> int:
    d = asdict(box)
    return d["a"].execute() + d["b"].run()


def use_dc_var(box: Box) -> int:
    d = dataclasses.asdict(box)
    return d["a"].execute() + d["b"].run()


def use_field_var(box: Box) -> int:
    d = asdict(box)
    xa = d["a"]
    xb = d["b"]
    return xa.execute() + xb.run()


def use_get(box: Box) -> int:
    d = asdict(box)
    return d.get("a").execute() + d.get("b").run()


def use_walrus(box: Box) -> int:
    if (d := asdict(box)):
        return d["a"].execute() + d["b"].run()
    return 0
