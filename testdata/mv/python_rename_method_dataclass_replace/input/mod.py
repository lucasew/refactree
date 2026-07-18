from dataclasses import dataclass, replace
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


def use_chain(box: Box) -> int:
    return replace(box).a.run() + replace(box).b.run()


def use_var(box: Box) -> int:
    new = replace(box)
    return new.a.run() + new.b.run()


def use_dc_chain(box: Box) -> int:
    return dataclasses.replace(box).a.run() + dataclasses.replace(box).b.run()


def use_dc_var(box: Box) -> int:
    new = dataclasses.replace(box)
    return new.a.run() + new.b.run()


def use_walrus(box: Box) -> int:
    if (new := replace(box)):
        return new.a.run() + new.b.run()
    return 0


def use_field_var(box: Box) -> int:
    xa = replace(box).a
    xb = replace(box).b
    return xa.run() + xb.run()
