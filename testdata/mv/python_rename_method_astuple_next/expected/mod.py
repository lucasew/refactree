from dataclasses import dataclass, astuple, replace
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


def use_next_iter_astuple(box: Box) -> int:
    return next(iter(astuple(box))).execute() + next(iter(astuple(box))).execute()


def use_next_astuple(box: Box) -> int:
    return next(astuple(box)).execute()


def use_dc_next_iter(box: Box) -> int:
    return next(iter(dataclasses.astuple(box))).execute()


def use_list_next(box: Box) -> int:
    return next(iter(list(astuple(box)))).execute()


def use_replace_next(box: Box) -> int:
    return next(iter(astuple(replace(box)))).execute()


def use_assign(box: Box) -> int:
    xa = next(iter(astuple(box)))
    xb = next(iter(dataclasses.astuple(box)))
    return xa.execute() + xb.execute()


def use_tuple_local(box: Box) -> int:
    t = astuple(box)
    return next(iter(t)).execute() + next(t).execute()


def use_list_local(box: Box) -> int:
    xs = list(astuple(box))
    xa = next(iter(xs))
    return xa.execute()


def use_index_still(box: Box) -> int:
    # already covered by astuple_direct; keep B leaf untouched
    return astuple(box)[1].run()
