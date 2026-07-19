import copy
from operator import attrgetter
from types import SimpleNamespace
import types


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_copy_attrgetter():
    return (
        copy.copy(attrgetter("k")(SimpleNamespace(k=A()))).run()
        + copy.copy(attrgetter("k")(SimpleNamespace(k=B()))).run()
    )


def use_deepcopy_attrgetter():
    return (
        copy.deepcopy(attrgetter("k")(SimpleNamespace(k=A()))).run()
        + copy.deepcopy(attrgetter("k")(SimpleNamespace(k=B()))).run()
    )


def use_copy_attrgetter_types():
    return (
        copy.copy(attrgetter("k")(types.SimpleNamespace(k=A()))).run()
        + copy.copy(attrgetter("k")(types.SimpleNamespace(k=B()))).run()
    )


def use_preserves_b():
    return copy.copy(attrgetter("k")(SimpleNamespace(k=B()))).run()
