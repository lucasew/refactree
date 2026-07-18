from dataclasses import dataclass, asdict, replace
from operator import itemgetter, attrgetter
import dataclasses
import operator


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


def use_ig_asdict(box: Box) -> int:
    return itemgetter("a")(asdict(box)).execute() + itemgetter("b")(asdict(box)).run()


def use_ag_replace(box: Box) -> int:
    return attrgetter("a")(replace(box)).execute() + attrgetter("b")(replace(box)).run()


def use_ig_dc(box: Box) -> int:
    return operator.itemgetter("a")(dataclasses.asdict(box)).execute() + operator.itemgetter("b")(dataclasses.asdict(box)).run()


def use_ag_dc(box: Box) -> int:
    return operator.attrgetter("a")(dataclasses.replace(box)).execute() + operator.attrgetter("b")(dataclasses.replace(box)).run()


def use_ig_var(box: Box) -> int:
    xa = itemgetter("a")(asdict(box))
    xb = itemgetter("b")(asdict(box))
    return xa.execute() + xb.run()


def use_ag_var(box: Box) -> int:
    xa = attrgetter("a")(replace(box))
    xb = attrgetter("b")(replace(box))
    return xa.execute() + xb.run()


def use_ig_vars_dict(box: Box) -> int:
    return itemgetter("a")(vars(box)).execute() + itemgetter("b")(box.__dict__).run()


def use_walrus(box: Box) -> int:
    if (xa := itemgetter("a")(asdict(box))):
        return xa.execute() + attrgetter("b")(replace(box)).run()
    return 0
