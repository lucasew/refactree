from typing import TypedDict


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class Box(TypedDict):
    a: A
    b: B


def use_sub(box: Box) -> int:
    return box["a"].execute() + box["b"].run()


def use_var(box: Box) -> int:
    xa = box["a"]
    xb = box["b"]
    return xa.execute() + xb.run()


def use_get(box: Box) -> int:
    xa = box.get("a")
    xb = box.get("b")
    return xa.execute() + xb.run()


def use_get_chain(box: Box) -> int:
    return box.get("a").execute() + box.get("b").run()
