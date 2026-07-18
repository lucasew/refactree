from dataclasses import dataclass, replace
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


def use_field_direct(box: Box) -> int:
    return copy.copy(box.a).execute() + copy.copy(box.b).run()


def use_field_assign(box: Box) -> int:
    xa = copy.copy(box.a)
    xb = copy.copy(box.b)
    return xa.execute() + xb.run()


def use_deepcopy_field(box: Box) -> int:
    return copy.deepcopy(box.a).execute() + copy.deepcopy(box.b).run()


def use_replace_field(box: Box) -> int:
    return copy.copy(replace(box).a).execute() + copy.copy(dataclasses.replace(box).b).run()


def use_item_direct(item: A, other: B) -> int:
    return copy.copy(item).execute() + copy.copy(other).run()


def use_item_assign(item: A, other: B) -> int:
    a = copy.copy(item)
    b = copy.copy(other)
    return a.execute() + b.run()


def use_import_copy(box: Box, item: A, other: B) -> int:
    from copy import copy, deepcopy
    return copy(box.a).execute() + deepcopy(box.b).run() + copy(item).execute() + deepcopy(other).run()
