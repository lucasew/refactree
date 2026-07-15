class Base:
    def assist(self):
        return 1


class Child(Base):
    def helper(self):
        return super().assist() + 1

    def stay(self):
        return 2
