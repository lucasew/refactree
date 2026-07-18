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


def use_asdict_get(box: Box) -> int:
    return asdict(box).get("a").run() + asdict(box).get("b").run()


def use_dc_asdict_get(box: Box) -> int:
    return dataclasses.asdict(box).get("a").run() + dataclasses.asdict(box).get("b").run()


def use_vars_get(box: Box) -> int:
    return vars(box).get("a").run() + vars(box).get("b").run()


def use_dunder_get(box: Box) -> int:
    return box.__dict__.get("a").run() + box.__dict__.get("b").run()


def use_field_var(box: Box) -> int:
    xa = asdict(box).get("a")
    xb = vars(box).get("b")
    return xa.run() + xb.run()


def use_dunder_field_var(box: Box) -> int:
    xa = box.__dict__.get("a")
    xb = box.__dict__.pop("b")
    return xa.run() + xb.run()
