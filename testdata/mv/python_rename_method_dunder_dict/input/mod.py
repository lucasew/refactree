from dataclasses import dataclass


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


def use_var(box: Box) -> int:
    d = box.__dict__
    return d["a"].run() + d["b"].run()


def use_field_var(box: Box) -> int:
    d = box.__dict__
    xa = d["a"]
    xb = d["b"]
    return xa.run() + xb.run()


def use_get(box: Box) -> int:
    d = box.__dict__
    return d.get("a").run() + d.get("b").run()


def use_walrus(box: Box) -> int:
    if (d := box.__dict__):
        return d["a"].run() + d["b"].run()
    return 0
