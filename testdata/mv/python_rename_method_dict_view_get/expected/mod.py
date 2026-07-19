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


def use_asdict_get(box: Box) -> int:
    return asdict(box).get("a").execute() + asdict(box).get("b").run()


def use_dc_asdict_get(box: Box) -> int:
    return dataclasses.asdict(box).get("a").execute() + dataclasses.asdict(box).get("b").run()


def use_vars_get(box: Box) -> int:
    return vars(box).get("a").execute() + vars(box).get("b").run()


def use_dunder_get(box: Box) -> int:
    return box.__dict__.get("a").execute() + box.__dict__.get("b").run()


def use_field_var(box: Box) -> int:
    xa = asdict(box).get("a")
    xb = vars(box).get("b")
    return xa.execute() + xb.run()


def use_dunder_field_var(box: Box) -> int:
    xa = box.__dict__.get("a")
    xb = box.__dict__.pop("b")
    return xa.execute() + xb.run()
