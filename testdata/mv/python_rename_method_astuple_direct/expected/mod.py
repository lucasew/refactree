from dataclasses import dataclass, astuple
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


def use_astuple(box: Box) -> int:
    return astuple(box)[0].execute() + astuple(box)[1].run()


def use_dc_astuple(box: Box) -> int:
    return dataclasses.astuple(box)[0].execute() + dataclasses.astuple(box)[1].run()


def use_field_var(box: Box) -> int:
    xa = astuple(box)[0]
    xb = dataclasses.astuple(box)[1]
    return xa.execute() + xb.run()
