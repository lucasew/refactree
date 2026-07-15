class Base:
    def assist(self):
        return 1

    def stay(self):
        return 2


class Child(Base):
    def run(self):
        return super().assist() + self.assist() + self.stay()
